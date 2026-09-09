package main

import (
	"context"
	"encoding/json"
	"fmt"

	"hostsent/backend/internal/pkg/upstream"
	"hostsent/backend/internal/pkg/upstream/mofangfinance"
)

func main() {
	cfg := &upstream.ProviderConfig{
		Type: mofangfinance.ProviderType, APIEndpoint: "https://www.haikayun.com/",
		APIKey: "3330989074@qq.com", APISecret: "oFgYrBFma0wH",
		UpstreamType: "zjmf_api", Timeout: 60,
	}
	p := mofangfinance.NewMoFangFinanceProvider(cfg)
	prods, err := p.ListProducts(context.Background())
	if err != nil { fmt.Println("ERR", err); return }
	fmt.Println("COUNT", len(prods))
	var price, spec int
	for _, pr := range prods {
		if pr.SalePrice > 0 { price++ }
		if pr.Specs.CPU > 0 || pr.Specs.Memory > 0 { spec++ }
	}
	fmt.Println("有价格", price, "/", len(prods), " 有规格(cpu/mem)", spec, "/", len(prods))
	for i, pr := range prods {
		if i >= 8 { break }
		b, _ := json.Marshal(map[string]interface{}{"id": pr.UpstreamID, "name": pr.Name, "price": pr.SalePrice,
			"cpu": pr.Specs.CPU, "memMB": pr.Specs.Memory, "disk": pr.Specs.Disk, "os": pr.Specs.OS, "region": pr.Specs.Region, "bw": pr.Specs.Bandwidth})
		fmt.Println(string(b))
	}
}
