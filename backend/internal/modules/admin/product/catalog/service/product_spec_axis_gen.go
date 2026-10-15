package service

// SPU → SKU 正交展开的生成实现（T4.7）。
//
// 一个商品（SPU）下按轴参数的笛卡尔积生成多个 SKU，每个 SKU：
//   - 原子取值 = 模板取值 + 该组合的轴取值（轴值覆盖模板同名参数）；
//   - 平台参数 = 模板平台参数 + 轴取值的写参数（轴值最后写入，保证轴说了算）；
//   - 编码/名称由组合**确定性派生**，因此重复执行"按模板生成"是幂等的
//     （同组合命中同一编码 → 更新，不新增），不会像时间戳后缀那样堆出重复 SKU。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/pkg/specatom"
)

// atomKeyByPlatformField 反查表：平台写字段名 → 原子 key。
//
// 轴参数名用的是**平台参数名**（cpu/memory/bw/system_disk_size），因为配置档的
// option_selections 就是按平台参数名勾的；而 SKU 的 specs 用**原子 key**
// （compute.cpu/compute.memory）。两边必须能互相翻译，否则轴取值进不了 SKU 基线，
// 开通时又会被平台的同名配置项覆盖回去（实测故障：名叫 6G、开出 4G）。
func atomKeyByPlatformField(platform string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(platform) == "" {
		return out
	}
	for _, atom := range specatom.Global().Atoms() {
		field, ok := atom.PlatformFields[platform]
		if !ok || strings.TrimSpace(field.Write) == "" {
			continue
		}
		if _, dup := out[field.Write]; !dup {
			out[field.Write] = atom.Key
		}
	}
	return out
}

// coerceAtomValue 按原子声明的类型把轴取值转成合适形态。
//
// 不转换的后果：compute.memory 存成字符串 "8192" 时 specatom 校验与
// WriteParams 都按字符串处理，平台侧收到 "8192" 而不是 8192，部分接口会拒。
func coerceAtomValue(atom specatom.Atom, raw string) any {
	switch atom.ValueType {
	case "int", "decimal":
		if n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			if atom.ValueType == "int" {
				return int(n)
			}
			return n
		}
	}
	return raw
}

// applyAxisToSpecValues 把轴组合写进原子取值 JSON，返回新的 JSON。
//
// 返回的 leftover 是"没有原子映射"的轴参数（如 port / snap_num 这类纯平台字段）：
// 它们进不了 specs，调用方需要直接写进平台参数，否则这个轴等于没生效。
func applyAxisToSpecValues(platform, specValuesJSON string, combo map[string]string) (string, map[string]string, error) {
	values := map[string]any{}
	if s := strings.TrimSpace(specValuesJSON); s != "" {
		if err := json.Unmarshal([]byte(s), &values); err != nil {
			return "", nil, fmt.Errorf("规格取值不是合法 JSON：%w", err)
		}
	}
	atomFields := atomKeyByPlatformField(platform)
	leftover := map[string]string{}
	keys := make([]string, 0, len(combo))
	for k := range combo {
		keys = append(keys, k)
	}
	// 稳定顺序：保证生成的 JSON 键序与日志可复现。
	sort.Strings(keys)
	for _, key := range keys {
		atomKey, ok := atomFields[key]
		if !ok {
			leftover[key] = combo[key]
			continue
		}
		atom, found := specatom.Global().Get(atomKey)
		if !found {
			leftover[key] = combo[key]
			continue
		}
		values[atomKey] = coerceAtomValue(atom, combo[key])
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", nil, err
	}
	return string(raw), leftover, nil
}

// applyAxisToPlatformParams 把轴取值（含无原子映射的 leftover）写进平台参数 JSON。
// 轴值最后写入，确保轴是这些键的最终真源。
func applyAxisToPlatformParams(platformParamsJSON string, combo, leftover map[string]string) (string, error) {
	params := map[string]any{}
	if s := strings.TrimSpace(platformParamsJSON); s != "" {
		if err := json.Unmarshal([]byte(s), &params); err != nil {
			return "", fmt.Errorf("平台参数不是合法 JSON：%w", err)
		}
	}
	keys := make([]string, 0, len(combo))
	for k := range combo {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		params[key] = combo[key]
	}
	for key, val := range leftover {
		params[key] = val
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// axisCodeSegment 轴取值 → 编码片段（可读且稳定）。
//
// CPU 与内存合成一段（4c6g），与 deriveSpecCode / deriveSpecCodeFromValues 的既有口径
// 一致；磁盘单独一段（100g）。不合成会得到 general-4c-6g-100g，而库里既有的同类规格
// 是 general-4c6g-100g —— 同一套约定的两种写法，运营看着像两批不同的东西。
func axisCodeSegment(atomKey, raw string) string {
	switch atomKey {
	case specatom.KeyCPU, specatom.KeyMemory:
		return "" // 由调用方合并为 NcNG 段
	case specatom.KeyDisk:
		return raw + "g"
	}
	return sanitizeCodeToken(atomKey) + sanitizeCodeToken(raw)
}

// combineCpuMemSegment 把轴组合里的 CPU 与内存拼成「NcNG」单段。
// 两者只有其一存在时退回单项写法（8c / 8g）。
func combineCpuMemSegment(atomFields map[string]string, combo map[string]string) string {
	cpuRaw := ""
	memRaw := ""
	for key, val := range combo {
		switch atomFields[key] {
		case specatom.KeyCPU:
			cpuRaw = val
		case specatom.KeyMemory:
			memRaw = val
		}
	}
	cpu, mem := 0, 0
	if v, err := strconv.ParseFloat(cpuRaw, 64); err == nil {
		cpu = int(v)
	}
	if v, err := strconv.ParseFloat(memRaw, 64); err == nil && v > 0 {
		mem = int(v / 1024)
		if mem <= 0 {
			mem = 1
		}
	}
	switch {
	case cpu > 0 && mem > 0:
		return strconv.Itoa(cpu) + "c" + strconv.Itoa(mem) + "g"
	case cpu > 0 && cpuRaw != "":
		return strconv.Itoa(cpu) + "c"
	case mem > 0 && memRaw != "":
		return strconv.Itoa(mem) + "g"
	}
	return ""
}

// sanitizeCodeToken 只保留编码安全字符（字母数字与 -），其余丢弃。
func sanitizeCodeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		}
	}
	return b.String()
}

// deriveAxisSpecCode 由规格族 + 轴组合派生确定性编码。
// 轴按 axisKeys 的顺序拼段，顺序固定 → 同一组合恒得同一编码。
// CPU 与内存合并为一段（4c6g），与既有编码口径一致。
func deriveAxisSpecCode(family string, platform string, axisKeys []string, combo map[string]string) string {
	atomFields := atomKeyByPlatformField(platform)
	parts := make([]string, 0, len(axisKeys)+1)
	if f := sanitizeCodeToken(strings.TrimSpace(family)); f != "" {
		parts = append(parts, f)
	}
	// CPU+内存合并段（若轴里包含它们）。
	if seg := combineCpuMemSegment(atomFields, combo); seg != "" {
		usedCpuMem := false
		for _, key := range axisKeys {
			switch atomFields[key] {
			case specatom.KeyCPU, specatom.KeyMemory:
				usedCpuMem = true
			}
		}
		if usedCpuMem {
			parts = append(parts, seg)
		}
	}
	for _, key := range axisKeys {
		segment := ""
		if atomKey, ok := atomFields[key]; ok {
			// CPU/内存已并入上一段，跳过。
			if atomKey == specatom.KeyCPU || atomKey == specatom.KeyMemory {
				continue
			}
			segment = axisCodeSegment(atomKey, combo[key])
		} else {
			segment = sanitizeCodeToken(key) + sanitizeCodeToken(combo[key])
		}
		if segment == "" {
			continue
		}
		parts = append(parts, segment)
	}
	code := strings.Join(parts, "-")
	// 全部轴都是 CPU/内存时 parts 可能只有族前缀（或全空）：补上合并段兜底。
	if len(parts) <= 1 {
		if seg := combineCpuMemSegment(atomFields, combo); seg != "" {
			if len(parts) == 0 {
				return seg
			}
			code = parts[0] + "-" + seg
		}
	}
	if code == "" {
		return "sku"
	}
	return code
}

// renderNameTemplate 用取值填充名称模板（"{cpu}核{memory}G {os}"）。
//
// 模板占位符与配置档参数名同口径，另支持内存按 G 展示：{memory} 取 GB，
// {memory_mb} 取 MB —— 与模板编辑器里"内存按 GB 填"的直觉一致。
func renderNameTemplate(tpl string, combo map[string]string, specValuesJSON string) string {
	out := strings.TrimSpace(tpl)
	if out == "" {
		return ""
	}
	values := map[string]any{}
	if s := strings.TrimSpace(specValuesJSON); s != "" {
		_ = json.Unmarshal([]byte(s), &values)
	}
	memoryGB := ""
	if mb := valueAsInt(values, specatom.KeyMemory); mb > 0 {
		memoryGB = strconv.Itoa(mb / 1024)
	}
	pairs := map[string]string{
		"cpu":       strconv.Itoa(valueAsInt(values, specatom.KeyCPU)),
		"memory":    memoryGB,
		"memory_mb": strconv.Itoa(valueAsInt(values, specatom.KeyMemory)),
		"disk":      strconv.Itoa(valueAsInt(values, specatom.KeyDisk)),
		"bw":        combo["bw"],
		"os":        combo["os"],
	}
	for key, val := range combo {
		if _, exists := pairs[key]; !exists {
			pairs[key] = val
		}
	}
	for key, val := range pairs {
		if val == "" {
			continue
		}
		out = strings.ReplaceAll(out, "{"+key+"}", val)
	}
	// 剩下的占位符（模板里写了但这次组合没有的参数）清掉，避免商品名带出 "{xxx}"。
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			break
		}
		out = out[:start] + out[start+end+1:]
	}
	return strings.Join(strings.Fields(out), " ")
}

// alignSpecValuesWithOptionDefaults 把"仍是客户选配项"的参数的基线值对齐到它声明的默认值。
//
// 为什么必须对齐（实测故障）：SKU 的原子基线来自模板结构字段（memory=6G），
// 而客户配置项的默认值来自配置档的 default（4G）。两者各记一份、互不校验，
// 结果就是商品名叫「4核6G」、变体编码写 6G，实际开通下发 memory=4096。
//
// 口径：一个参数既然开放给客户选，它的**默认选择**就决定了不带选配时开出来的规格 ——
// 所以 SKU 基线必须等于该默认值，两个真源合并成一个。
// 轴参数不在此列：轴取值已由组合写入，且轴参数根本不生成客户配置项。
func alignSpecValuesWithOptionDefaults(platform, specValuesJSON string, sel map[string]optionSelection) (string, error) {
	values := map[string]any{}
	if s := strings.TrimSpace(specValuesJSON); s != "" {
		if err := json.Unmarshal([]byte(s), &values); err != nil {
			return "", fmt.Errorf("规格取值不是合法 JSON：%w", err)
		}
	}
	atomFields := atomKeyByPlatformField(platform)
	for _, key := range sortedOptionKeys(sel) {
		item := sel[key]
		if item.SKUAxis {
			continue
		}
		def := strings.TrimSpace(item.Default.String())
		if def == "" {
			continue
		}
		atomKey, ok := atomFields[key]
		if !ok {
			continue
		}
		atom, found := specatom.Global().Get(atomKey)
		if !found {
			continue
		}
		values[atomKey] = coerceAtomValue(atom, def)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// generateAxisSpecsFromTemplate 把一个模板选择按轴展开成多个 SKU。
//
// 幂等：编码由组合确定性派生，重复执行时同编码命中既有 SKU → 更新而不是新增，
// 解决"重复生成堆出一批同名 SKU"的问题。
func (s *productService) generateAxisSpecsFromTemplate(
	ctx context.Context,
	productID uint64,
	product *model.Product,
	sel dto.ProductSpecTemplateSelection,
	tpl *SpecTemplateSnapshot,
	plan *axisPlan,
	operatorID uint64,
	operatorName string,
) ([]dto.ProductSpecInfo, error) {
	specValues := strings.TrimSpace(string(sel.SpecValues))
	if specValues == "" {
		specValues = strings.TrimSpace(tpl.SpecValues)
	}
	if specValues == "" {
		specValues = deriveSpecValuesFromTemplate(tpl)
	}
	// 非轴参数的基线对齐到它声明的默认值（见 alignSpecValuesWithOptionDefaults 注释）：
	// 让"SKU 基线"与"客户配置项默认值"合并成一个真源。
	selections := parseOptionSelections(tpl.OptionSelections)
	aligned, aerr := alignSpecValuesWithOptionDefaults(tpl.ProviderType, specValues, selections)
	if aerr != nil {
		return nil, fmt.Errorf("规格模板「%s」：%w", tpl.Name, aerr)
	}
	specValues = aligned
	platformParams := strings.TrimSpace(string(sel.PlatformParams))
	if platformParams == "" {
		platformParams = strings.TrimSpace(tpl.PlatformParams)
	}
	axisKeys := plan.axisKeys()
	out := make([]dto.ProductSpecInfo, 0, len(plan.Combinations))
	for _, combo := range plan.Combinations {
		mergedSpecs, leftover, err := applyAxisToSpecValues(tpl.ProviderType, specValues, combo)
		if err != nil {
			return out, fmt.Errorf("规格模板「%s」：%w", tpl.Name, err)
		}
		if err := validateSpecJSON(mergedSpecs); err != nil {
			return out, fmt.Errorf("规格模板「%s」：%w", tpl.Name, err)
		}
		mergedParams, err := applyAxisToPlatformParams(platformParams, combo, leftover)
		if err != nil {
			return out, fmt.Errorf("规格模板「%s」：%w", tpl.Name, err)
		}
		code := deriveAxisSpecCode(tpl.SpecFamily, tpl.ProviderType, axisKeys, combo)
		name := renderNameTemplate(tpl.NameTemplate, combo, mergedSpecs)
		if name == "" {
			name = tpl.Name
		}
		price := sel.Price
		if price <= 0 {
			price = tpl.Price
		}
		if price <= 0 {
			price = product.Price
		}
		stock := sel.Stock
		if stock == 0 {
			stock = -1
		}
		item := &model.ProductSpec{
			ProductID:      productID,
			SpecCode:       code,
			Name:           name,
			Specs:          mergedSpecs,
			PriceModel:     firstNonEmptyStr(product.PriceModel, model.PriceModelFixed),
			Price:          price,
			CostPrice:      sel.CostPrice,
			Stock:          stock,
			SpecTemplateID: tpl.ID,
			Status:         model.ProductSpecEnabled,
		}
		// 同组合已有 SKU → 更新（幂等重跑），否则新增。
		// 不做时间戳后缀：那会让每次重跑都多出一批同名 SKU（实测故障）。
		if existing, ferr := s.repo.FindSpecByCode(ctx, productID, code); ferr == nil && existing != nil {
			item.ID = existing.ID
			item.SortOrder = existing.SortOrder
			if err := s.repo.UpdateSpec(ctx, item); err != nil {
				return out, fmt.Errorf("规格模板「%s」更新 SKU %s 失败：%w", tpl.Name, code, err)
			}
		} else if ferr != nil && !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return out, ferr
		} else if err := s.repo.CreateSpec(ctx, item); err != nil {
			return out, fmt.Errorf("规格模板「%s」生成 SKU %s 失败：%w", tpl.Name, code, err)
		}
		// 平台绑定：合并后的参数即该 SKU 的边界写参数。
		if strings.TrimSpace(mergedParams) != "" && s.bindingWriter != nil {
			_ = s.bindingWriter.UpsertConfirmedProductSpecBinding(
				ctx, item.ID, tpl.ID, mergedParams, operatorID,
				"按规格模板生成（轴组合）："+tpl.Name)
		}
		infos := []dto.ProductSpecInfo{buildProductSpecInfo(*item)}
		s.fillSpecBinding(ctx, infos)
		out = append(out, infos[0])
	}
	if len(out) > 0 {
		_ = s.repo.AddHistory(ctx, &model.ProductHistory{
			ProductID: productID, ChangeType: model.ChangeTypeUpdate,
			NewValue: fmt.Sprintf("%d", len(out)), OperatorID: operatorID, OperatorName: operatorName,
			Remark: fmt.Sprintf("按规格模板「%s」正交生成 %d 个规格变体", tpl.Name, len(out)),
		})
	}
	return out, nil
}
