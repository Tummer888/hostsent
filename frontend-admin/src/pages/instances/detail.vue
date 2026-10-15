<template>
  <div class="page-body instances-module">
    <!-- ============ 页头：只放动作条，不重复身份信息 ============
      此前这里还有标题（主机名）、电源/服务/来源三个标签、以及"实例标识 · 公网 IP ·
      归属"一行 —— 与下方主信息区**逐字重复**：同一屏里主机名出现两次、状态出现三次、
      归属出现三次（用户实测反馈"显示运行中的地方就有三个"）。

      身份与状态统一由主信息区（detail-hero）承担，它是"认机器"的唯一入口；
      本页头退化为纯动作条，避免运营在两个地方读同一件事、也避免两处不一致
      （例如页头没跟着刷新时，两处状态会互相矛盾）。 -->
    <header class="page-header page-header--actions surface-card">
      <!-- 操作分组：日常 / 电源 / 维护 / 危险，避免十几个按钮平铺挤成一团 -->
      <!--
        操作区：按「用的频率 + 危险度」从左到右排，组间加竖线分隔。
        此前是 9 个同款描边按钮平铺，看不出哪个是常用的；现在：
          · 控制台/电源 —— 日常最高频，给实心主按钮；
          · 维护/账期与凭证/变配/续费 —— 中等频率，保持描边；
          · 危险操作 —— 红色留在最右；
          · 刷新与同步刷新 —— 只读动作，退到最右的次要位置。
        待开通实例只留「重新开通」：其余动作没有对象，隐藏而不是让它们点了报错。
      -->
      <div class="detail-actions">
        <template v-if="isPending">
          <t-button v-permission="'instance:action'" theme="primary" :loading="retrying" @click="handleRetryProvision">
            重新开通
          </t-button>
        </template>
        <template v-else>
          <t-button
            v-permission="'instance:console'"
            theme="primary"
            :disabled="consoleDisabled"
            :title="consoleDisabled ? consoleTitle : ''"
            @click="handleVnc"
          >
            <template #icon><DesktopIcon aria-hidden="true" /></template>
            控制台
          </t-button>

          <!-- 电源操作收进下拉：硬电源（断电级）在组内用红色标注 -->
          <t-dropdown :options="powerOptions" trigger="click" @click="onPowerPick">
            <t-button theme="primary" variant="outline" :disabled="powerGroupDisabled"> 电源操作 ▾ </t-button>
          </t-dropdown>

          <span class="actions-divider" aria-hidden="true"></span>

          <!-- 维护类（平台接口实测存在）：重装 / 重置密码 / 救援 / 快照 / 硬件 -->
          <t-dropdown :options="maintenanceOptions" trigger="click" @click="onMaintenancePick">
            <t-button variant="outline" :disabled="maintenanceDisabled">维护 ▾</t-button>
          </t-dropdown>

          <t-button v-permission="'lifecycle:renew'" variant="outline" @click="openRenewDialog">续费</t-button>
          <t-button
            v-permission="'instance:resize'"
            variant="outline"
            :disabled="!instanceData?.capabilities?.resize"
            :title="capabilityTitle(instanceData?.capabilities?.resize)"
            @click="openResizeDialog"
          >
            变配
          </t-button>

          <!-- 账期与凭证：本地口径操作与敏感查看，独立分组 -->
          <t-dropdown :options="billingOptions" trigger="click" @click="onBillingPick">
            <t-button variant="outline" :disabled="billingDisabled">账期与凭证 ▾</t-button>
          </t-dropdown>

          <span class="actions-divider" aria-hidden="true"></span>

          <!-- 危险区：暂停/恢复 + 销毁 -->
          <t-dropdown :options="dangerOptions" trigger="click" @click="onDangerPick">
            <t-button theme="danger" variant="outline" :disabled="dangerDisabled">危险操作 ▾</t-button>
          </t-dropdown>
        </template>

        <span class="actions-divider" aria-hidden="true"></span>
        <t-button
          v-if="!isPending"
          v-permission="'instance:action'"
          variant="text"
          :loading="syncing"
          @click="handleSync"
        >
          <!-- 回源要等平台响应（网络往返 + 远端查询），图标转起来让"正在同步"可见。
               措辞按链路：自营实例的机器由本平台开通，"同步上游"会让运营以为
               背后还有个第三方（用户实测反馈同类问题）。 -->
          <template #icon>
            <RefreshIcon aria-hidden="true" :class="{ 'state-spin-icon': syncing }" />
          </template>
          {{ resale ? '同步上游' : '同步状态' }}
        </t-button>
        <t-button variant="text" :loading="loading" @click="reloadAll">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button variant="text" @click="goBack">返回</t-button>
      </div>
    </header>

    <t-alert v-if="isPending" theme="error" :title="provisionAlertTitle" :message="provisionAlertMessage" />

    <!--
      过渡态提示条（迁移 112）：上游正在执行某个动作时，把"在做什么、从什么时候开始"
      明确摊开。没有这条提示时，运营看不到任何变化就会反复点同一个按钮，
      同一台机器堆了多次重启（用户实测反馈）。
      两个阶段分开措辞：窗口内是"进行中，请等待"；超时后是"结果未确认，先同步"。
    -->
    <t-alert v-else-if="transitionActive" theme="info" class="transition-alert">
      <template #title>
        <span class="transition-alert__title">
          <LoadingIcon size="14" class="state-spin-icon" />
          实例正在{{ transition?.verb }}
        </span>
      </template>
      <template #message>
        <div class="transition-alert__body">
          <span class="transition-alert__text">
            {{
              transition?.started_at ? `开始于 ${formatTime(transition.started_at)}，` : ''
            }}{{ resale ? '上游' : '平台' }}指令为异步执行，通常需要 1–3 分钟。期间电源与维护操作暂时不可用，已完成
            {{ transitionElapsedText }}。
          </span>
          <!--
            进度条：用户明确要求"从开机变到关机的过程中加一个关机的加载动画"。
            只转一个小图标不够显眼（一屏里容易漏看），加一条不确定态流动条，
            让"正在执行"在一眼扫过时就成立。宽度用 CSS 动画而不是真实百分比 ——
            上游不回报进度（见遗留项 31），画一个假百分比会误导。
          -->
          <span class="transition-progress" role="progressbar" :aria-label="`正在${transition?.verb}`"></span>
        </div>
      </template>
    </t-alert>
    <t-alert v-else-if="transitionOverdue" theme="warning" title="上次操作的结果尚未确认">
      <template #message>
        <span>
          {{ transition?.verb }}指令已下发但超过预计耗时仍未确认（可能仍在执行，也可能回执丢失）。
          请先点「{{ resale ? '同步上游' : '同步状态' }}」核对实际状态，再决定是否需要重新操作。
        </span>
      </template>
    </t-alert>
    <t-alert
      v-else-if="instanceData?.capability_error"
      theme="warning"
      :message="`部分操作不可用：${instanceData.capability_error}`"
    />

    <!--
      ============ 主信息区（一屏之内回答"这是台什么机器、怎么登"） ============
      运维在实例详情页最常做的三件事：认机器（主机名/实例 ID）、确认交付参数
      （配置/公网 IP/端口/账号/系统）、看备注（客户说了什么）。这三块此前散在
      速览条与「概览」tab 的描述表里，反而让到期/付款等财务信息占了首屏 ——
      本区把它们提到最上面，且不再用带彩色左边框的卡片（那种"演示占位"观感
      容易让人误以为是空态），改为同一张卡内的分区栅格 + 分隔线。
    -->
    <section v-if="instanceData" class="detail-hero surface-card">
      <!-- 身份行：主机名（= 平台侧 hostname）+ 两个正交状态 + 云主机 ID -->
      <header class="hero-head">
        <div class="hero-identity">
          <OsIcon :os="instanceData.os" size="lg" />
          <div class="hero-identity__text">
            <div class="hero-identity__name">
              <span class="hero-hostname">{{ instanceData.name || '未命名主机' }}</span>
              <!--
                状态 = 两个独立维度，各回答一个问题（本轮修复的核心）：
                  · 电源维度：这台机器通电了吗（运行中/已关机/重启中…）
                  · 服务维度：我们卖的账期还有效吗（服务中/即将到期/到期暂停…）
                此前是一个合成标签并把"关机"写成「已暂停」—— 因为上游把 suspended
                也映射成 stopped，两者无法区分（用户实测反馈）。

                这里是**全页唯一**的状态展示位：此前页头也挂一份，"运行中"在同屏
                出现三次（用户实测反馈），且两处可能不同步（页头未刷新时互相矛盾）。
              -->
              <t-tag
                :theme="headlinePower.theme"
                variant="light"
                size="small"
                shape="round"
                :class="{ 'state-spinning': headlinePower.spinning }"
              >
                <LoadingIcon v-if="headlinePower.spinning" size="13" class="state-spin-icon" />
                {{ headlinePower.label }}
              </t-tag>
              <t-tag
                :theme="headlineService.theme"
                variant="light"
                size="small"
                shape="round"
                :title="headlineService.detail"
              >
                {{ headlineService.label }}
              </t-tag>
            </div>
            <div class="hero-identity__meta">
              <!--
                实例号按链路分开表达（用户实测反馈："虽然是我自己的，但它这里会显示
                上游实例号，这个一看就是有问题"）：

                  · 转售链路（resale）—— 有两个号，必须分清：
                    本地实例号 instance_id（本系统的主键性质标识，历史流水引用它）
                    与上游实例号 provider_instance_id（发给上游做电源/控制台/续费/销毁，
                    上游换机后这个号会变，故提供「重新关联」）。

                  · 自营链路 —— 只有一个号：机器由本平台开通，这个号就是本系统
                    生成并下发的。显示「上游实例号」会让运营以为背后还有个第三方，
                    进而去点「重新关联」填一个来路不明的号（后端已拒绝，但界面
                    本就不该给出这个入口）。
              -->
              <span class="hero-id">
                {{ resale ? '本地实例号' : '实例号' }}
                <code>{{ instanceData.instance_id || '—' }}</code>
                <span class="hero-id__tag">{{ resale ? '本系统' : '本平台' }}</span>
              </span>
              <span v-if="resale" class="hero-id">
                上游实例号
                <template v-if="instanceData.provider_instance_id">
                  <code>{{ instanceData.provider_instance_id }}</code>
                  <span class="hero-id__tag hero-id__tag--upstream">{{ instanceData.provider_name || '上游' }}</span>
                </template>
                <template v-else>
                  <code class="hero-id__pending">未开通（占位中）</code>
                </template>
                <t-link
                  v-if="hasPerm('instance:relink') && !instanceData.pending"
                  theme="primary"
                  hover="color"
                  class="hero-id__action"
                  @click="openRelinkDialog"
                >
                  重新关联
                </t-link>
              </span>
              <span class="hero-id hero-id--muted">
                <!--
                  归属客户 = 这台机器属于谁（下钻到客户详情）。
                  用户详情是「下钻页」，路由为 /users/accounts/detail?id=N（query 传参）：
                  此前这里写成路径参数 /users/accounts/{id}，与任何路由都不匹配 ——
                  点击后落到空白页（用户实测）。

                  带"归属"二字而不是裸用户名：裸链接夹在一排技术标识（本地号/上游号）
                  之间看不出是什么（用户实测反馈"归属也有一点重复"）。归属全页只此
                  一处，概览描述表里的重复项已移除。
                -->
                <span class="hero-id__label">归属</span>
                <router-link
                  class="hero-user"
                  :to="{ path: '/users/accounts/detail', query: { id: String(instanceData.user_id) } }"
                  >{{ instanceData.username || '—' }}</router-link
                >
                <template v-if="instanceData.actor_name">（{{ instanceData.actor_name }} 开通）</template>
              </span>
            </div>
          </div>
        </div>
        <div class="hero-tags">
          <t-tag :theme="sourceModeTheme" variant="light" size="small" shape="round">{{ sourceModeLabel }}</t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round">{{
            providerTypeLabel(instanceData.provider_type)
          }}</t-tag>
        </div>
      </header>

      <!-- 交付参数栅格：配置 / 网络 / 登录 / 系统，四块最常被问的信息 -->
      <div class="hero-grid">
        <!-- 配置 -->
        <div class="hero-cell">
          <div class="hero-cell__label"><CpuIcon aria-hidden="true" />配置规格</div>
          <div class="hero-cell__value">
            <span class="hero-num">{{ instanceData.cpu }}</span
            ><span class="hero-unit">核</span>
            <span class="hero-sep">/</span>
            <span class="hero-num">{{ formatMemory(instanceData.memory) }}</span>
            <span class="hero-sep">/</span>
            <span class="hero-num">{{ instanceData.disk }}</span
            ><span class="hero-unit">GB{{ instanceData.disk_type ? ` ${instanceData.disk_type}` : '' }}</span>
          </div>
          <div class="hero-cell__sub">
            带宽 <b>{{ instanceData.bandwidth }}</b> Mbps
            <span v-if="instanceData.region || instanceData.zone" class="hero-sep">·</span>
            <span v-if="instanceData.region || instanceData.zone">{{
              [instanceData.region, instanceData.zone].filter(Boolean).join(' / ')
            }}</span>
          </div>
        </div>

        <!-- 网络：公网 IP 是最关键信息，用等宽大字。
             NAT 机型没有独立 IP，主位改显转发地址——只写"未分配"会让运营
             以为机器没配好，实际是机型就没这个能力。 -->
        <div class="hero-cell">
          <div class="hero-cell__label"><LinkIcon aria-hidden="true" />网络地址</div>
          <template v-if="isNatInstance">
            <div class="hero-cell__value">
              <span v-if="natPrimaryAddress" class="hero-ip">{{ natPrimaryAddress }}</span>
              <span v-else class="hero-ip hero-ip--muted">暂无转发地址</span>
              <t-button
                v-if="natPrimaryAddress"
                size="small"
                variant="text"
                theme="primary"
                class="hero-copy"
                @click="copyText(natPrimaryAddress, 'NAT 访问地址')"
              >
                复制
              </t-button>
            </div>
            <div class="hero-cell__sub">
              <t-tag theme="warning" variant="light" size="small" shape="round">NAT 机型</t-tag>
              <span>不分配独立 IP，经节点转发访问</span>
            </div>
          </template>
          <template v-else>
            <div class="hero-cell__value">
              <span class="hero-ip">{{ instanceData.public_ip || '未分配' }}</span>
              <t-button
                v-if="instanceData.public_ip"
                size="small"
                variant="text"
                theme="primary"
                class="hero-copy"
                @click="copyText(instanceData.public_ip, '公网 IP')"
              >
                复制
              </t-button>
            </div>
            <div class="hero-cell__sub">
              内网 {{ instanceData.private_ip || '—' }}
              <span v-if="instanceData.power_status" class="hero-sep">·</span>
              <span v-if="instanceData.power_status">电源 {{ powerStatusText(instanceData.power_status) }}</span>
            </div>
          </template>
        </div>

        <!-- 登录凭据：用户名 + 端口 + 密码入口 -->
        <div class="hero-cell">
          <div class="hero-cell__label"><UserIcon aria-hidden="true" />登录凭据</div>
          <div class="hero-cell__value">
            <span class="hero-user-name">{{ instanceData.credential?.username || '—' }}</span>
            <span class="hero-port">端口 {{ instanceData.credential?.port || '—' }}</span>
          </div>
          <div class="hero-cell__sub">
            <template v-if="instanceData.credential?.has_password">
              <t-link v-if="hasPerm('instance:credential')" theme="primary" hover="color" @click="openRevealDialog">
                查看密码
              </t-link>
              <span v-else class="hero-hint">密码已保存（需「查看密码」权限）</span>
            </template>
            <span v-else class="hero-hint">{{ instanceData.credential?.password_hint || '未保存密码，请重置' }}</span>
            <span v-if="instanceData.credential?.has_panel_password" class="hero-hint"> · 另有面板/IPMI 密码</span>
            <!--
              重置密码就近放在凭据格：这是"密码不好用"时的第一反应动作。
              此前只藏在「维护」下拉里（用户实测反馈"详情里找不到重置密码"）。
              能力位与待开通判据都带上，避免点了必然失败。
            -->
            <template v-if="hasPerm('instance:password')">
              <span class="hero-sep">·</span>
              <t-link
                theme="primary"
                hover="color"
                :disabled="!instanceData.capabilities?.reset_password || isPending"
                @click="openResetPasswordDialog"
              >
                重置密码
              </t-link>
            </template>
          </div>
        </div>

        <!-- 操作系统：图标 + 版本 + 重装入口（换系统是高频操作，就近给按钮） -->
        <div class="hero-cell">
          <div class="hero-cell__label"><DesktopIcon aria-hidden="true" />操作系统</div>
          <div class="hero-cell__value">
            <OsIcon :os="instanceData.os" />
            <span class="hero-os">{{ osDisplayName(instanceData.os) }}</span>
            <!--
              能力位不支持时保留按钮但置灰（而不是直接隐藏）：
              运营需要看到"为什么没有重装入口"，title 里给出原因。
            -->
            <t-button
              v-if="!isPending"
              v-permission="'instance:reinstall'"
              size="small"
              variant="text"
              theme="primary"
              class="hero-copy"
              :disabled="!instanceData.capabilities?.reinstall"
              :title="instanceData.capabilities?.reinstall ? '' : '该服务商不支持重装系统'"
              @click="openReinstallDialog"
            >
              重装
            </t-button>
          </div>
          <div class="hero-cell__sub">镜像/模板由平台下发；重装会清空系统盘</div>
        </div>
      </div>

      <!-- 备注两栏：客户备注（客户说的）与管理员备注（运营记的）并排，避免互相覆盖 -->
      <div class="hero-notes">
        <div class="hero-note" :class="{ 'hero-note--empty': !instanceData.customer_remark }">
          <span class="hero-note__label"><ChatIcon aria-hidden="true" />客户备注</span>
          <span class="hero-note__text">{{ instanceData.customer_remark || '客户未填写' }}</span>
          <span
            v-if="
              instanceData.order?.customer_remark && instanceData.order.customer_remark !== instanceData.customer_remark
            "
            class="hero-note__sub"
          >
            下单时填写：{{ instanceData.order.customer_remark }}
          </span>
        </div>
        <div class="hero-note" :class="{ 'hero-note--empty': !instanceData.remark }">
          <span class="hero-note__label"><EditIcon aria-hidden="true" />管理员备注</span>
          <span class="hero-note__text">{{ instanceData.remark || '未填写' }}</span>
          <t-link
            v-permission="'instance:action'"
            theme="primary"
            hover="color"
            class="hero-note__edit"
            @click="openRemarkDialog"
          >
            编辑
          </t-link>
        </div>
      </div>

      <!-- 账期条：财务口径压成一行（原速览条里占 6 张卡，现只占一行） -->
      <div class="hero-billing">
        <div class="hero-billing__item" :class="expireGlanceClass">
          <span class="hero-billing__label">到期时间</span>
          <span class="hero-billing__value">
            {{ instanceData.expire_at ? formatTime(instanceData.expire_at) : isOnetime ? '一次性/买断' : '未设置' }}
          </span>
          <span
            v-if="instanceData.expire_at && instanceData.expire_state !== 'none'"
            class="hero-billing__sub"
            :class="expireClass(instanceData)"
          >
            {{ expireStateLabel(instanceData.expire_state) }} · 剩余 {{ instanceData.days_left }} 天
          </span>
        </div>
        <div class="hero-billing__item">
          <span class="hero-billing__label">付款周期</span>
          <span class="hero-billing__value">{{ billingModeLabel }}</span>
        </div>
        <div class="hero-billing__item">
          <span class="hero-billing__label">首次付款</span>
          <span class="hero-billing__value">¥{{ formatPrice(instanceData.order?.first_paid_amount ?? 0) }}</span>
        </div>
        <div class="hero-billing__item">
          <span class="hero-billing__label">续费金额</span>
          <span class="hero-billing__value hero-billing__value--strong"
            >¥{{ formatPrice(instanceData.order?.renew_amount ?? 0) }}</span
          >
          <!-- 绑定价压过商品价：不标出来，运营会以为看到的是商品价（调完价也看不出变化） -->
          <span v-if="instanceData.order?.renew_bound" class="hero-billing__sub">
            已绑定谈定价<template v-if="instanceData.renewal_price"
              >（¥{{ formatPrice(instanceData.renewal_price) }} / 周期）</template
            >
          </span>
          <span
            v-else-if="resale && instanceData.upstream && instanceData.order?.upstream_renew_amount"
            class="hero-billing__sub"
          >
            上游 ¥{{ formatPrice(instanceData.order.upstream_renew_amount) }}
          </span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <!-- ============ 概览 ============ -->
        <t-tab-panel value="overview" label="概览">
          <div class="tabs-section">
            <!--
              明细表：**只放主信息区没有的东西**。
              主信息区（detail-hero）已经展示：主机名、电源态+服务态、两个实例号、
              归属客户（含开通人）、配置/网络/凭据/系统、客户与管理员备注、到期时间、
              付款周期、首付/续费金额、服务商、来源。
              因此这里刻意**不再**重复状态标签与归属用户名 —— 同屏出现两三次同一信息
              既啰嗦又容易不一致（用户实测反馈"运行中的地方有三个""归属也重复"）。
              留下的是"排查才看"的原始值：上游状态码、生命周期阶段、回源时间、
              客户联系方式（主信息区只给用户名链接，联系方式在这里补全）。
            -->
            <t-descriptions v-if="instanceData" :column="3" size="medium" class="detail-desc">
              <t-descriptions-item label="上游状态">
                <span class="cell-strong">{{ instanceStatusLabel(instanceData.status) }}</span>
                <div class="cell-sub">
                  原始值 {{ instanceData.status || '—' }} · 阶段
                  {{ lifecycleStageLabel(instanceData.lifecycle_stage) }}{{
                    lifecycleNotice ? `（${lifecycleNotice}）` : ''
                  }}
                </div>
              </t-descriptions-item>
              <t-descriptions-item label="来源断言">
                <span class="cell-strong">{{ sourceModeLabel }}</span>
                <div class="cell-sub">{{ sourceModeHint }}</div>
              </t-descriptions-item>
              <t-descriptions-item label="服务商">
                {{ instanceData.provider_name || '—' }}
                <span v-if="instanceData.provider_type" class="cell-sub"
                  >（{{ providerTypeLabel(instanceData.provider_type) }}）</span
                >
              </t-descriptions-item>
              <t-descriptions-item label="客户联系方式">
                <span v-if="instanceData.user_email || instanceData.user_phone">
                  <template v-if="instanceData.user_email">{{ instanceData.user_email }}</template
                  ><template v-if="instanceData.user_email && instanceData.user_phone"> · </template
                  ><template v-if="instanceData.user_phone">{{ instanceData.user_phone }}</template>
                </span>
                <span v-else class="cell-sub">未登记</span>
                <div class="cell-sub">
                  <router-link
                    class="inline-link"
                    :to="{ path: '/users/accounts/detail', query: { id: String(instanceData.user_id) } }"
                  >
                    查看客户资料
                  </router-link>
                </div>
              </t-descriptions-item>
              <t-descriptions-item label="最近回源">{{ formatTime(instanceData.last_synced_at) }}</t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(instanceData.created_at) }}</t-descriptions-item>
              <t-descriptions-item
                v-if="instanceData.enforce_attempts > 0 || instanceData.last_enforce_error"
                label="到期处置"
                :span="3"
              >
                <span class="expire-danger">连续失败 {{ instanceData.enforce_attempts }} 次</span>
                <div v-if="instanceData.enforce_next_at" class="cell-sub">
                  下次重试：{{ formatTime(instanceData.enforce_next_at) }}
                </div>
                <div v-if="instanceData.last_enforce_error" class="cell-sub op-error">
                  {{ instanceData.last_enforce_error }}
                </div>
              </t-descriptions-item>
            </t-descriptions>

            <!-- 订单 / 商品 / 上游 三栏关联信息 -->
            <div class="info-grid">
              <div class="info-card">
                <div class="info-card__head">
                  <h3 class="card-title">来源订单</h3>
                  <t-link
                    v-if="instanceData?.order"
                    theme="primary"
                    hover="color"
                    @click="openOrderById(instanceData.order.id)"
                  >
                    查看订单
                  </t-link>
                </div>
                <template v-if="instanceData?.order">
                  <div class="kv-row">
                    <span class="kv-row__k">订单号</span
                    ><span class="kv-row__v">{{ instanceData.order.order_no }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">订单状态</span
                    ><span class="kv-row__v">{{ orderStatusLabel(instanceData.order.status) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">商品名称</span
                    ><span class="kv-row__v">{{ instanceData.order.product_name || '—' }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">订购时间</span
                    ><span class="kv-row__v">{{ formatTime(instanceData.order.ordered_at) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">订购数量</span
                    ><span class="kv-row__v">{{ instanceData.order.quantity }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">付款周期</span
                    ><span class="kv-row__v">{{ cycleLabel(instanceData.order.cycle) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">订单金额</span
                    ><span class="kv-row__v">¥{{ formatPrice(instanceData.order.total_amount) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">优惠减免</span
                    ><span class="kv-row__v">¥{{ formatPrice(instanceData.order.discount_amount) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">首次付款</span
                    ><span class="kv-row__v kv-row__v--strong"
                      >¥{{ formatPrice(instanceData.order.first_paid_amount) }}</span
                    >
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">支付方式</span
                    ><span class="kv-row__v">{{ instanceData.order.pay_method || '—' }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">付款时间</span
                    ><span class="kv-row__v">{{ formatTime(instanceData.order.pay_time) }}</span>
                  </div>
                </template>
                <t-empty v-else description="无来源订单" />
              </div>

              <!--
                商品与对接：**一张卡按链路分支**，不再并排「对接商品 / 上游商品」两张。
                原因（实测数据）：对接商品的 products 行是从资源镜像生成的，
                name/price/cost_price 与 resource_products 的名称、标价、采购价逐字相同
                （见 doc"商品与上游重复"一节），并排两张卡会把同一个名字同一组价格
                各显示一遍。这里改为：自营只讲我方商品口径；对接则讲「我方售价 →
                上游成本」的对照关系，只在两处名称真的不同（运营改过名）时才额外提示原名。
              -->
              <div class="info-card info-card--span">
                <div class="info-card__head">
                  <h3 class="card-title">{{ resale ? '商品与对接' : '商品信息' }}</h3>
                  <div class="info-card__actions">
                    <!--
                      换绑入口按链路分派（用户实测反馈："我自定的商品，但它还显示
                      对接商品"）：

                        · 转售链路：上游换产品（五区→六区）时本地商品必须跟着换，
                          否则续费按旧价算 —— 给出「更换对接商品」。
                        · 自营链路：没有"对接商品"这回事，入口文案改为
                          「更换本地商品」；对话框内也不会出现上游镜像选项
                          （后端候选接口对自营只返回本地商品）。
                    -->
                    <t-link
                      v-if="hasPerm('instance:billing') && !instanceData?.pending"
                      theme="primary"
                      hover="color"
                      @click="openRebindDialog"
                    >
                      {{ resale ? '更换对接商品' : '更换本地商品' }}
                    </t-link>
                    <t-link
                      v-if="instanceData?.product"
                      theme="primary"
                      hover="color"
                      @click="openProductById(instanceData.product.id)"
                    >
                      查看商品
                    </t-link>
                    <t-tag
                      v-if="instanceData?.product"
                      :theme="instanceData.product.source_mode === 'upstream' ? 'warning' : 'success'"
                      variant="light"
                      size="small"
                      shape="round"
                    >
                      {{ productSourceModeLabel(instanceData.product.source_mode) }}
                    </t-tag>
                    <t-tag
                      v-if="instanceData?.upstream"
                      :theme="instanceData.upstream.renewable ? 'success' : 'warning'"
                      variant="light"
                      size="small"
                      shape="round"
                    >
                      {{ instanceData.upstream.renewable ? '可续费' : '不可续费' }}
                    </t-tag>
                  </div>
                </div>

                <template v-if="instanceData?.product">
                  <t-alert
                    v-if="instanceData.upstream?.provider_mismatch"
                    theme="warning"
                    class="dlg-alert"
                    :message="`该镜像所属渠道（${instanceData.upstream.provider_name || `#${instanceData.upstream.provider_id}`}）与实例渠道不一致：多为历史数据/重新导入遗留，开通与续费请以实例渠道为准`"
                  />

                  <!-- 商品身份：名称只出现一次 -->
                  <div class="kv-row">
                    <span class="kv-row__k">商品名称</span>
                    <span class="kv-row__v">
                      <b>{{ instanceData.product.name }}</b>
                      <t-tag v-if="instanceData.product.deleted" theme="danger" variant="light-outline" size="small"
                        >商品行已删除</t-tag
                      >
                      <t-tag
                        v-else
                        :theme="instanceData.product.status === 1 ? 'success' : 'default'"
                        variant="light"
                        size="small"
                        shape="round"
                      >
                        {{ instanceData.product.status === 1 ? '上架' : '下架' }}
                      </t-tag>
                      <div v-if="resale && upstreamRenamed" class="cell-sub">
                        上游原名：{{ instanceData.upstream?.name }}
                      </div>
                    </span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">商品编码</span
                    ><span class="kv-row__v">{{ instanceData.product.code || '—' }}</span>
                  </div>

                  <!-- 对接关系：仅转售链路才有（自营不对接上游镜像商品） -->
                  <template v-if="resale && instanceData.upstream">
                    <div class="kv-row">
                      <span class="kv-row__k">对接渠道</span>
                      <span class="kv-row__v">
                        {{ instanceData.upstream.provider_name || instanceData.product.source_provider_name || '—' }}
                        <span v-if="instanceData.upstream.provider_mismatch" class="cell-sub"
                          >（商品镜像渠道为 {{ instanceData.product.source_provider_name || '—' }}）</span
                        >
                      </span>
                    </div>
                    <div class="kv-row">
                      <span class="kv-row__k">上游商品</span>
                      <span class="kv-row__v">
                        #{{ instanceData.upstream.upstream_id || '—' }}
                        <span class="cell-sub">
                          （本地镜像 #{{
                            instanceData.upstream.resource_product_id || instanceData.product.source_product_id || '—'
                          }}<template v-if="instanceData.upstream.group_name">
                            · {{ instanceData.upstream.group_name }}</template
                          >）
                        </span>
                      </span>
                    </div>
                  </template>
                  <div v-else-if="resale" class="kv-row">
                    <span class="kv-row__k">对接信息</span>
                    <span class="kv-row__v cell-sub"
                      >本地镜像已不存在（重新导入/清理后常见）：上游价格与对接关系无法回溯</span
                    >
                  </div>

                  <!--
                    价格对照：我方售价/成本与上游标价/采购价在对接链路上完全相同，
                    因此只用一行「售价 → 成本」表达，并在两者不等时标注上游口径，
                    不再并列两组同名数字。毛利与加价率现场算出，方便运营判断调价空间。
                  -->
                  <div class="kv-row">
                    <span class="kv-row__k">售价 / 成本</span>
                    <span class="kv-row__v">
                      <span class="kv-row__v--price">¥{{ formatPrice(instanceData.product.price) }}</span>
                      <span class="price-arrow">→</span>
                      <span class="kv-row__v--price">{{
                        instanceData.product.cost_price > 0 ? `¥${formatPrice(instanceData.product.cost_price)}` : '—'
                      }}</span>
                      <t-tag v-if="grossMarginText" theme="success" variant="light" size="small" shape="round">{{
                        grossMarginText
                      }}</t-tag>
                      <div v-if="resale && instanceData.upstream" class="cell-sub">
                        商品成本即上游采购价{{
                          upstreamCostDiffers
                            ? `（上游现价 ¥${formatPrice(instanceData.upstream.cost_price)}，与本地不一致，续费前请核对）`
                            : '，与上游一致'
                        }}
                      </div>
                    </span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">加价规则</span
                    ><span class="kv-row__v">{{ markupLabel(instanceData.product) }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-row__k">续费（售价 / 成本）</span>
                    <span class="kv-row__v">
                      <span class="kv-row__v--strong">¥{{ formatPrice(instanceData.order?.renew_amount ?? 0) }}</span>
                      <span class="price-arrow">/</span>
                      <span>¥{{ formatPrice(instanceData.order?.renew_cost ?? 0) }}</span>
                      <span class="cell-sub">按 {{ cycleLabel(instanceData.order?.renew_cycle) }}</span>
                      <t-tag
                        v-if="instanceData.order?.renew_bound"
                        theme="warning"
                        variant="light"
                        size="small"
                        shape="round"
                        >已绑定谈定价</t-tag
                      >
                    </span>
                  </div>

                  <!-- 上游各周期价：对接链路才有，是调价时的唯一依据 -->
                  <div v-if="resale && instanceData.upstream" class="kv-row kv-row--col">
                    <span class="kv-row__k">上游各周期价（标价 / 采购价）</span>
                    <div class="cycle-chips">
                      <span v-for="item in upstreamCycleRows" :key="item.cycle" class="cycle-chip">
                        <b>{{ cycleLabel(item.cycle) }}</b>
                        <span>¥{{ formatPrice(item.list) }} / ¥{{ formatPrice(item.cost) }}</span>
                      </span>
                      <span v-if="!upstreamCycleRows.length" class="cell-sub">上游未提供周期价</span>
                    </div>
                    <div v-if="instanceData.upstream.discount_desc" class="cell-sub">
                      代理折扣：{{ instanceData.upstream.discount_desc }}
                    </div>
                  </div>
                </template>

                <t-empty
                  v-else
                  :description="
                    resale
                      ? '未找到售出商品记录（上游同步发现的实例，或商品行已删除）'
                      : '该实例非经我方商品开通'
                  "
                />
              </div>
            </div>
          </div>
        </t-tab-panel>

        <!-- ============ 配置与财务 ============ -->
        <t-tab-panel value="billing" label="配置与财务">
          <div class="tabs-section">
            <div class="config-grid">
              <!-- 生效配置项（区域/CPU/内存/带宽等） -->
              <div class="config-panel">
                <div class="config-panel__head">
                  <h3 class="card-title">生效配置</h3>
                  <span class="table-card__meta">共 {{ configs.length }} 项 · 以实例实际取值为准</span>
                </div>
                <!--
                  本地选配 vs 上游选配必须分清（用户实测反馈）：同样叫"配置项"，
                  但一边是我们商品里客户选的档位（改它要动商品/SKU），
                  另一边是上游商品钉死的参数（我们改不动，只能去上游面板改）。
                  给两个来源各一句带商品名的说明，比只打「上游固定」标签更明确。
                -->
                <div v-if="instanceData?.upstream || instanceData?.product" class="config-legend">
                  <span v-if="instanceData?.product" class="config-legend__item">
                    <span class="config-legend__dot config-legend__dot--local"></span>
                    {{ resale ? '本地选配' : '商品选配' }}：{{ instanceData.product.name
                    }}<span class="cell-sub">（我们商品里的档位/选配项）</span>
                  </span>
                  <!-- 上游选配：仅转售链路才有（自营没有"上游商品钉死的参数"这一层） -->
                  <span v-if="resale && instanceData?.upstream" class="config-legend__item">
                    <span class="config-legend__dot config-legend__dot--upstream"></span>
                    上游选配：{{ instanceData.upstream.name }}<span class="cell-sub">（上游商品钉死的参数）</span>
                  </span>
                </div>
                <div v-if="configs.length" class="config-list">
                  <div v-for="item in configs" :key="item.key" class="config-item">
                    <div class="config-item__head">
                      <span class="config-item__label">{{ item.label }}</span>
                      <t-tag v-if="item.locked" theme="warning" variant="light" size="small" shape="round"
                        >上游锁定</t-tag
                      >
                      <t-tag v-else :theme="configSourceTheme(item.source)" variant="light" size="small" shape="round">
                        {{ configSourceLabel(item.source) }}
                      </t-tag>
                    </div>
                    <div class="config-item__value">
                      {{ item.value || '—' }}<span v-if="item.unit" class="config-item__unit">{{ item.unit }}</span>
                    </div>
                  </div>
                </div>
                <t-empty v-else description="该商品未配置可选项" />
              </div>

              <!--
                NAT 访问地址（doc122）：仅 NAT 机型展示。这类机器没有独立 IP，
                客户只能经节点转发访问；转发规则可能有多个（SSH/网站/数据库各一条），
                所以在这里列全量而不是只给主位那一个地址。
              -->
              <div v-if="isNatInstance" class="config-panel">
                <div class="config-panel__head">
                  <h3 class="card-title">NAT 访问地址</h3>
                  <span class="table-card__meta">
                    {{ natAccess?.forwards?.length || 0 }} 条转发 · {{ natAccess?.domains?.length || 0 }} 个建站域名
                  </span>
                </div>
                <!-- 读取失败 vs 没有规则是两回事：前者可重试，后者是配置状态 -->
                <t-alert v-if="natError" theme="warning" class="nat-alert">
                  <template #message>{{ natError }}</template>
                </t-alert>
                <div v-if="natAccess?.forwards?.length || natAccess?.domains?.length" class="config-list">
                  <div v-for="addr in natAccess?.forwards || []" :key="`f-${addr}`" class="config-item nat-item">
                    <div class="config-item__head">
                      <span class="config-item__label">端口转发</span>
                      <t-link theme="primary" hover="color" size="small" @click="copyText(addr, 'NAT 地址')"
                        >复制</t-link
                      >
                    </div>
                    <div class="config-item__value">
                      <code>{{ addr }}</code>
                    </div>
                  </div>
                  <div v-for="domain in natAccess?.domains || []" :key="`d-${domain}`" class="config-item nat-item">
                    <div class="config-item__head">
                      <span class="config-item__label">共享建站</span>
                      <t-link theme="primary" hover="color" size="small" @click="copyText(domain, '建站域名')"
                        >复制</t-link
                      >
                    </div>
                    <div class="config-item__value">
                      <code>{{ domain }}</code>
                    </div>
                  </div>
                </div>
                <t-empty v-else-if="!natError" description="该实例暂无转发规则（可在平台面板创建后刷新）" />
              </div>

              <!--
                成本与账期调整：到期时间/付款周期/首付/续费售价已在主信息区账期条里，
                这里不重复；只放主信息区没展开的成本侧口径与"调整账期"入口。
              -->
              <div class="config-panel">
                <div class="config-panel__head">
                  <h3 class="card-title">成本与账期调整</h3>
                  <t-button v-permission="'instance:billing'" size="small" variant="outline" @click="openAdjustDialog">
                    调整账期
                  </t-button>
                </div>
                <div class="kv-row">
                  <span class="kv-row__k">计费链路</span
                  ><span class="kv-row__v"
                    >{{ sourceModeLabel }}<span class="cell-sub">{{ sourceModeHint }}</span></span
                  >
                </div>
                <div class="kv-row">
                  <span class="kv-row__k">续费成本</span
                  ><span class="kv-row__v"
                    >¥{{ formatPrice(instanceData?.order?.renew_cost ?? 0)
                    }}<span class="cell-sub">供应商侧成本，售价见主信息区</span></span
                  >
                </div>
                <div v-if="resale && instanceData?.upstream" class="kv-row">
                  <span class="kv-row__k">上游续费金额</span>
                  <span class="kv-row__v kv-row__v--strong">{{ upstreamRenewText }}</span>
                </div>
                <div class="kv-row">
                  <span class="kv-row__k">来源订单</span
                  ><span class="kv-row__v"
                    >{{ instanceData?.order?.order_no || '—'
                    }}<t-link
                      v-if="instanceData?.order"
                      theme="primary"
                      hover="color"
                      class="inline-link"
                      @click="openOrderById(instanceData.order.id)"
                      >查看</t-link
                    ></span
                  >
                </div>
                <t-alert
                  v-if="resale"
                  theme="warning"
                  class="billing-alert"
                  message="该实例对接上游，到期时间与付款周期的最终口径以上游为准；本地调整需勾选「强制本地调整」，且不会被回源覆盖后保持。"
                />
                <t-alert
                  v-else
                  theme="info"
                  class="billing-alert"
                  message="自营实例的到期时间与付款周期由本系统管理，可直接在「调整账期」中自定义。"
                />
              </div>
            </div>
          </div>
        </t-tab-panel>

        <!-- ============ 操作记录 ============ -->
        <t-tab-panel value="operations" label="操作记录">
          <div class="tabs-section">
            <div class="table-card__head">
              <span class="table-card__meta">共 {{ opPagination.total }} 条</span>
            </div>
            <t-table
              row-key="id"
              :data="operations"
              :columns="operationColumns"
              :loading="opLoading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
              :pagination="opPagination"
              @page-change="handleOpPageChange"
            >
              <template #action="{ row }">
                <span>{{ operationActionLabel(row.action) }}</span>
              </template>
              <template #result="{ row }">
                <t-tag :theme="operationResultTheme(row.result)" variant="light" size="small" shape="round">
                  {{ operationResultLabel(row.result) }}
                </t-tag>
              </template>
              <template #operator="{ row }">
                <div>
                  <span class="cell-strong">{{ operatorTypeLabel(row.operator_type) }}</span>
                  <div class="cell-sub">{{ row.operator_name || '—' }}</div>
                </div>
              </template>
              <template #status_change="{ row }">
                <span class="cell-muted">{{ row.before_status || '—' }}</span>
                <span class="status-arrow">→</span>
                <span class="cell-muted">{{ row.after_status || '—' }}</span>
              </template>
              <template #detail="{ row }">
                <t-tooltip v-if="row.result === 'failed' && row.error_message" :content="row.error_message">
                  <span class="op-error">{{ row.error_message }}</span>
                </t-tooltip>
                <span v-else>—</span>
              </template>
              <template #created_at="{ row }">
                <span class="time-text">{{ formatTime(row.created_at) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无操作记录" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <!-- ============ 关联记录 ============ -->
        <t-tab-panel value="related" label="关联记录">
          <div class="tabs-section">
            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联订单</h3>
                <span class="table-card__meta">共 {{ related.orders.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.orders"
                :columns="relatedOrderColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #order_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openOrder(row)">{{ row.order_no }}</t-link>
                </template>
                <template #total_amount="{ row }">
                  <span>¥{{ formatPrice(row.total_amount) }}</span>
                </template>
                <template #paid_amount="{ row }">
                  <span>¥{{ formatPrice(row.paid_amount) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联订单" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">续费记录</h3>
                <span class="table-card__meta">共 {{ related.renewals.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.renewals"
                :columns="relatedRenewalColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #amount="{ row }">
                  <span>¥{{ formatPrice(row.amount) }}</span>
                </template>
                <template #expire_before="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_before) }}</span>
                </template>
                <template #expire_after="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_after) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无续费记录" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联工单</h3>
                <span class="table-card__meta">共 {{ related.tickets.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.tickets"
                :columns="relatedTicketColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #ticket_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openTicket(row)">{{ row.ticket_no }}</t-link>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联工单" />
                </template>
              </t-table>
            </div>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- ============ 弹窗 ============ -->
    <t-dialog
      v-model:visible="remarkVisible"
      header="实例备注"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveRemark"
      @close="remarkVisible = false"
    >
      <t-form label-align="top" :data="remarkForm" @submit.prevent>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="remarkForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入实例备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 账期调整：到期时间 + 付款周期，仅本地口径不产生账单 -->
    <t-dialog
      v-model:visible="adjust.visible"
      header="调整账期与付款周期"
      width="680px"
      :confirm-btn="{ content: '确认调整', theme: 'primary', disabled: adjustConfirmDisabled }"
      :cancel-btn="{ content: '取消' }"
      destroy-on-close
      @confirm="handleAdjustBilling"
    >
      <t-alert
        theme="info"
        class="dlg-alert"
        message="调整只改本地账期口径，不产生账单、不向上游下单。对接上游的实例需勾选强制本地调整。"
      />
      <t-alert
        v-if="adjustCtx?.upstream_cycles_unavailable"
        theme="warning"
        class="dlg-alert"
        message="未能读到上游支持的计费周期（渠道不可用）：下列周期为平台全集，**未经上游核对**，选中上游不卖的周期会导致账单周期错位。"
      />
      <t-form label-align="top">
        <t-form-item label="到期时间">
          <t-date-picker
            v-model="adjust.expire_at"
            enable-time-picker
            clearable
            placeholder="选择新的到期时间（留空表示不修改）"
            class="dlg-full"
          />
        </t-form-item>
        <!--
          付款周期：选项**由后端按上游能力下发**（AdjustContext），不再前端硬编码。
          此前硬编码 9 档（含小时/按日/两年/三年），选一个上游不卖的周期会推出
          与上游对不上的账单周期（用户实测缺陷）。
        -->
        <t-form-item label="付款周期">
          <t-select
            v-model="adjust.billing_mode"
            :options="adjustCycleOptions"
            :loading="adjustCtxLoading"
            clearable
            placeholder="留空表示不修改"
            class="dlg-full"
            @change="onAdjustCycleChange"
          />
          <!-- 选中周期后即时比价：上游标价/采购价与我方售价/成本并排 -->
          <div v-if="selectedCycleRow" class="cycle-compare">
            <div class="cycle-compare__row">
              <span class="cycle-compare__k">上游标价</span>
              <span class="cycle-compare__v">{{
                selectedCycleRow.upstream_list ? `¥${formatPrice(selectedCycleRow.upstream_list)}` : '—'
              }}</span>
              <span class="cycle-compare__k">上游采购价</span>
              <span class="cycle-compare__v">{{
                selectedCycleRow.upstream_cost ? `¥${formatPrice(selectedCycleRow.upstream_cost)}` : '—'
              }}</span>
            </div>
            <div class="cycle-compare__row">
              <span class="cycle-compare__k">我方售价</span>
              <span class="cycle-compare__v">{{
                selectedCycleRow.has_product_price ? `¥${formatPrice(selectedCycleRow.sale)}` : '未配置'
              }}</span>
              <span class="cycle-compare__k">我方成本</span>
              <span class="cycle-compare__v">{{
                selectedCycleRow.has_product_price ? `¥${formatPrice(selectedCycleRow.cost)}` : '未配置'
              }}</span>
            </div>
            <div v-if="selectedCycleMarginText" class="cycle-compare__margin">{{ selectedCycleMarginText }}</div>
            <t-button
              v-if="selectedCycleRow.upstream_cost > 0"
              size="small"
              variant="text"
              theme="primary"
              @click="fillRenewalFromUpstream"
            >
              按上游采购价填入续费价
            </t-button>
          </div>
        </t-form-item>
        <!-- 绑定续费单价：实例详情页此前无法调整续费价（用户实测反馈） -->
        <t-form-item label="绑定续费价（元/周期）">
          <div class="pwd-input">
            <t-input-number
              v-model="adjust.renewal_price"
              :min="0"
              :decimal-places="2"
              placeholder="留空表示不修改；填 -1 取消绑定"
              class="dlg-grow"
            />
            <t-button variant="outline" :disabled="!adjust.renewal_price" @click="clearAdjustRenewalPrice"
              >取消绑定</t-button
            >
          </div>
          <div class="cell-sub">
            {{ adjustCtx?.renewal_price_hint || '绑定价压过商品调价与折扣；留空表示本次不改。' }}
            <template v-if="currentRenewalPriceText"> 当前：{{ currentRenewalPriceText }}</template>
          </div>
        </t-form-item>
        <t-form-item v-if="resale">
          <t-checkbox v-model="adjust.force">强制本地调整（该实例为上游对接链路）</t-checkbox>
        </t-form-item>
        <t-form-item label="调整原因">
          <t-textarea v-model="adjust.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，写入操作流水" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!--
      按上游实例号重新关联：上游换机/换号后，本地留的旧号会让此后所有上游动作
      （电源/控制台/续费/销毁）打在已不存在的机器上。填入上游新号 → 先探活再落库。
    -->
    <t-dialog
      v-model:visible="relink.visible"
      header="按上游实例号重新关联"
      width="560px"
      :confirm-btn="{ content: '重新关联', theme: 'warning', disabled: relinkConfirmDisabled }"
      :cancel-btn="{ content: '取消' }"
      :confirm-loading="relink.loading"
      destroy-on-close
      @confirm="handleRelink"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="重新关联后，本实例的电源/控制台/续费/销毁等**所有上游动作**都会作用到新号对应的机器上。请确认新号就是上游正在为本客户运行的那台。"
      />
      <t-form label-align="top">
        <t-form-item label="当前上游实例号">
          <code>{{ instanceData?.provider_instance_id || '（未开通）' }}</code>
          <span class="cell-sub">渠道：{{ instanceData?.provider_name || '—' }}</span>
        </t-form-item>
        <t-form-item label="目标上游实例号">
          <t-input v-model="relink.upstream_instance_id" placeholder="从上游面板复制实例 ID（纯数字，如 2057）" />
          <div class="cell-sub">提交前会向上游探活：号码不存在会被拒绝，不会把实例指向虚空。</div>
        </t-form-item>
        <t-form-item v-if="relinkCanMigrateLocal" label="本地实例号">
          <t-checkbox v-model="relink.migrate_instance_id">
            同时把本地实例号改为新上游号（当前两者一致，改后可保持语义统一）
          </t-checkbox>
          <div v-if="!relink.migrate_instance_id && !relink.migrateDone" class="cell-sub">
            不改则：本地实例号保持 <code>{{ instanceData?.instance_id }}</code
            >（历史流水仍可对应）
          </div>
        </t-form-item>
        <t-form-item label="原因">
          <t-input v-model="relink.reason" placeholder="如：上游换机，旧号 2057 → 新号 2103（写入操作流水）" />
        </t-form-item>
      </t-form>
      <!-- 关联成功后回显从上游拉到的信息，运营一眼核对确实是对的那台 -->
      <div v-if="relinkResult" class="relink-result">
        <div class="relink-result__title">已关联，从上游读回：</div>
        <div class="kv-row">
          <span class="kv-row__k">主机名</span><span class="kv-row__v">{{ relinkResult.name || '—' }}</span>
        </div>
        <div class="kv-row">
          <span class="kv-row__k">状态</span
          ><span class="kv-row__v">{{ instanceStatusLabel(relinkResult.status) }}</span>
        </div>
        <div class="kv-row">
          <span class="kv-row__k">公网 IP</span><span class="kv-row__v">{{ relinkResult.public_ip || '—' }}</span>
        </div>
        <div class="kv-row">
          <span class="kv-row__k">内网 IP</span><span class="kv-row__v">{{ relinkResult.private_ip || '—' }}</span>
        </div>
        <div class="kv-row">
          <span class="kv-row__k">系统 / 区域</span
          ><span class="kv-row__v">{{ osDisplayName(relinkResult.os) }} · {{ relinkResult.region || '—' }}</span>
        </div>
      </div>
      <p class="dlg-confirm-tip">
        请输入当前实例标识 <code>{{ instanceData?.instance_id }}</code> 以确认：
      </p>
      <t-input v-model="relink.confirm_mark" :placeholder="instanceData?.instance_id || '当前实例标识'" />
    </t-dialog>

    <!--
      更换绑定的本地商品 / 上游镜像：上游换了产品（如五区→六区），
      本地商品不跟着换，续费会按旧价算、配置也按旧口径推。
    -->
    <t-dialog
      v-model:visible="rebind.visible"
      header="更换对接商品"
      width="600px"
      :confirm-btn="{ content: '确认更换', theme: 'warning' }"
      :cancel-btn="{ content: '取消' }"
      :confirm-loading="rebind.loading"
      destroy-on-close
      @confirm="handleRebind"
    >
      <!--
        对话框同样按链路分派（用户实测反馈："自营的商品就不存在更换对接商品"）：
        自营链路没有上游镜像这一层 —— 不显示上游镜像行、不提供「更换上游镜像」
        复选框，措辞也去掉"上游"（后端对自营会拒绝该字段，前端不该先给出选项）。
      -->
      <t-alert
        v-if="resale"
        theme="warning"
        class="dlg-alert"
        message="更换直接决定**续费算价**与推给上游的配置口径。上游换了商品时，本地商品必须同步换过来，否则两边账单对不上。"
      />
      <t-alert
        v-else
        theme="info"
        class="dlg-alert"
        message="更换只影响**续费算价**与展示口径（该实例为自营链路，机器由本平台开通，不涉及上游）。"
      />
      <t-form label-align="top">
        <t-form-item label="当前绑定">
          <div class="cell-sub">
            本地商品：<b>{{ instanceData?.product?.name || '—' }}</b>
            <template v-if="instanceData?.product?.id">（#{{ instanceData.product.id }}）</template>
          </div>
          <div v-if="resale" class="cell-sub">
            上游镜像：<b>{{ instanceData?.upstream?.name || '（未关联）' }}</b>
            <template v-if="instanceData?.upstream"
              >（#{{ instanceData.upstream.resource_product_id }} · 上游号
              {{ instanceData.upstream.upstream_id }}）</template
            >
          </div>
        </t-form-item>
        <t-form-item label="变更项">
          <t-checkbox-group v-model="rebind.fields">
            <t-checkbox value="sell">更换本地售出商品</t-checkbox>
            <t-checkbox v-if="resale" value="upstream">更换上游镜像</t-checkbox>
            <t-checkbox value="renewal_price">同时更新续费价</t-checkbox>
          </t-checkbox-group>
          <div class="cell-sub">
            {{
              resale
                ? '未勾选的项保持不动；三项可分开做（如只换本地商品、上游镜像已经是对的）。'
                : '未勾选的项保持不动；自营链路只能更换本地售出商品与续费价。'
            }}
          </div>
        </t-form-item>
        <t-form-item v-if="rebind.fields.includes('sell')" label="新的本地售出商品">
          <!-- 选框替代手填 ID：候选由后端给出（含上下架/价格/已配置周期），
               运营不必先去商品列表查号再回来填（用户实测反馈）。 -->
          <t-select
            v-model="rebind.sell_product_id"
            :options="rebindSellOptions"
            :loading="rebindCandidatesLoading"
            filterable
            clearable
            placeholder="搜索商品名或编码选择"
            class="dlg-full"
          />
          <div class="cell-sub">决定续费算价与展示口径。必须是已上架、同渠道的商品。</div>
        </t-form-item>
        <t-form-item v-if="rebind.fields.includes('upstream')" label="新的本地上游镜像">
          <t-select
            v-model="rebind.upstream_product_id"
            :options="rebindUpstreamOptions"
            :loading="rebindCandidatesLoading"
            filterable
            clearable
            placeholder="仅列出本实例同渠道的镜像，可搜索"
            class="dlg-full"
          />
          <div class="cell-sub">
            决定上游价与配置来源；只列出同渠道镜像（跨渠道会被拒绝）。
            <template v-if="rebindHint"> {{ rebindHint }}</template>
          </div>
        </t-form-item>
        <t-form-item v-if="rebind.fields.includes('renewal_price')" label="新的绑定续费价（元/周期）">
          <t-input-number
            v-model="rebind.renewal_price"
            :min="0"
            :decimal-places="2"
            placeholder="留空 = 清除绑定（回到按商品实时算价）"
            class="dlg-full"
          />
        </t-form-item>
        <t-form-item label="原因">
          <t-input v-model="rebind.reason" placeholder="如：上游把五区商品换成六区，同步本地（写入操作流水）" />
        </t-form-item>
      </t-form>
      <div v-if="rebindResult" class="relink-result">
        <div class="relink-result__title">已更换：</div>
        <div class="kv-row">
          <span class="kv-row__k">本地商品</span
          ><span class="kv-row__v"
            >#{{ rebindResult.old_sell_product_id || '—' }} → <b>#{{ rebindResult.new_sell_product_id || '—' }}</b>
            {{ rebindResult.sell_product_name || '' }}</span
          >
        </div>
        <div class="kv-row">
          <span class="kv-row__k">上游镜像</span
          ><span class="kv-row__v"
            >#{{ rebindResult.old_upstream_product_id || '—' }} →
            <b>#{{ rebindResult.new_upstream_product_id || '—' }}</b>
            {{ rebindResult.upstream_product_name || '' }}</span
          >
        </div>
      </div>
    </t-dialog>

    <!-- 明文凭据查看：点击后即时拉取，不随详情缓存 -->
    <t-dialog
      v-model:visible="reveal.visible"
      header="查看登录密码"
      width="480px"
      :confirm-btn="{ content: '我已记录', theme: 'primary' }"
      :cancel-btn="null"
      destroy-on-close
      @confirm="reveal.visible = false"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="查看密码会写入操作审计流水。请勿截图外传，使用完毕请及时关闭。"
      />
      <t-descriptions :column="1" bordered size="small">
        <t-descriptions-item label="用户名">{{ reveal.username || '—' }}</t-descriptions-item>
        <t-descriptions-item label="密码">
          <span class="secret-cell">
            <code>{{ reveal.password || '—' }}</code>
            <t-button size="small" variant="text" theme="primary" @click="copyRevealPassword">复制</t-button>
          </span>
        </t-descriptions-item>
        <t-descriptions-item label="端口">{{ reveal.port || '—' }}</t-descriptions-item>
      </t-descriptions>
    </t-dialog>

    <t-dialog
      v-model:visible="resizeVisible"
      header="变配"
      width="520px"
      :confirm-btn="{ content: '提交变配', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleResize"
      @close="resizeVisible = false"
    >
      <t-form label-align="top" :data="resizeForm" @submit.prevent>
        <t-form-item label="CPU（核）" name="cpu">
          <t-input-number v-model="resizeForm.cpu" :min="1" :max="256" theme="column" />
        </t-form-item>
        <t-form-item label="内存（MB）" name="memory">
          <t-input-number v-model="resizeForm.memory" :min="128" :step="128" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘（GB）" name="disk">
          <t-input-number v-model="resizeForm.disk" :min="1" :step="10" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘类型" name="disk_type">
          <t-input v-model="resizeForm.disk_type" placeholder="如 cloud_ssd" />
        </t-form-item>
        <t-form-item label="变配原因" name="reason">
          <t-textarea
            v-model="resizeForm.reason"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，请填写变配原因"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="destroyVisible"
      header="销毁实例"
      width="520px"
      :confirm-btn="{ content: '确认销毁', theme: 'danger', disabled: destroyConfirmDisabled }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleDestroy"
      @close="destroyVisible = false"
    >
      <t-form label-align="top" :data="destroyForm" @submit.prevent>
        <t-form-item>
          <t-alert
            theme="error"
            :message="`销毁后实例与数据不可恢复。请输入实例标识「${instanceData?.instance_id || ''}」以确认。`"
          />
        </t-form-item>
        <t-form-item label="实例标识确认" name="confirm_mark">
          <t-input v-model="destroyForm.confirm_mark" :placeholder="instanceData?.instance_id || '请输入实例标识'" />
        </t-form-item>
        <t-form-item label="销毁原因" name="reason">
          <t-textarea
            v-model="destroyForm.reason"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，请填写销毁原因"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="renewVisible"
      header="续费"
      width="520px"
      :confirm-btn="{ content: '提交续费', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRenew"
      @close="renewVisible = false"
    >
      <t-form label-align="top" :data="renewForm" @submit.prevent>
        <t-form-item label="续费时长（周期）" name="period_count">
          <t-input-number v-model="renewForm.period_count" :min="1" :max="120" theme="column" />
        </t-form-item>
        <t-form-item label="金额（元）" name="amount">
          <t-input-number
            v-model="renewForm.amount"
            :min="0"
            :precision="2"
            theme="column"
            placeholder="留空按默认价格"
          />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea
            v-model="renewForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，请填写代续费备注"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!--
      控制台凭据弹窗：VNC 改为**新标签页**打开（用户反馈：不要用弹窗内嵌）。
      弹窗内嵌的问题：控制台是独立会话，弹窗关闭即断开；且窗口尺寸受限、
      无法与实例页并排操作。这里只负责"把密码给运营 + 提示去新标签页"，
      真正的控制台在浏览器标签页里跑（可固定、可多开、与实例页并排）。
    -->
    <t-dialog
      v-model:visible="vnc.visible"
      header="控制台已在新标签页打开"
      width="480px"
      :confirm-btn="{ content: '再次打开', theme: 'primary' }"
      :cancel-btn="{ content: '知道了' }"
      destroy-on-close
      @confirm="reopenVnc"
      @close="vnc.visible = false"
    >
      <t-alert
        theme="info"
        class="dlg-alert"
        message="控制台是本机远程画面，建议在独立标签页中操作（可固定该标签页，回来不影响会话）。"
      />
      <div v-if="vnc.password" class="vnc-meta">
        <span class="vnc-meta__label">控制台密码</span>
        <code class="vnc-meta__value">{{ vnc.password }}</code>
        <t-button size="small" variant="text" theme="primary" @click="copyVncPassword">复制</t-button>
      </div>
      <p v-if="vncBlocked" class="vnc-blocked">
        浏览器拦截了自动打开的新标签页（多为弹窗拦截）。请点击下方按钮手动打开。
      </p>
      <div v-if="vnc.url" class="vnc-url">
        <span class="cell-sub">控制台地址：{{ vnc.url }}</span>
      </div>
      <div class="vnc-actions">
        <t-button v-if="vncBlocked" theme="primary" @click="openVnc">手动打开控制台</t-button>
        <t-button v-if="!isHttp(vnc.url)" variant="outline" @click="copyVncUrl">复制地址</t-button>
      </div>
    </t-dialog>

    <!-- 维护类操作弹窗：重装 / 重置密码 / 救援 / 快照 / 硬件 -->
    <t-dialog
      v-model:visible="reinstall.visible"
      header="重装系统"
      :confirm-btn="{ content: '发起重装', theme: 'danger' }"
      :confirm-disabled="!reinstall.os"
      destroy-on-close
      @confirm="handleReinstall"
    >
      <t-alert theme="warning" class="dlg-alert" message="重装会清空系统盘（可选同时格式化数据盘），操作不可撤销。" />
      <t-form label-align="top">
        <t-form-item label="目标镜像">
          <t-select
            v-model="reinstall.os"
            filterable
            clearable
            creatable
            :loading="mirrorLoading"
            :options="mirrorOptions"
            placeholder="从列表选择（支持搜索），也可直接填镜像 ID"
          />
          <!-- 镜像来源与取值口径：两种链路的 ID 含义不同，不讲清楚运营不敢选 -->
          <div class="cell-sub">
            <template v-if="mirrorSource === 'product'">
              镜像取自该商品的操作系统配置项，共 {{ mirrorOptions.length }} 个；下发给上游的是配置子项
              ID（与开通口径一致）
            </template>
            <template v-else-if="mirrorSource === 'platform'">
              镜像取自渠道平台镜像目录，共 {{ mirrorOptions.length }} 个
            </template>
            <template v-else>
              {{ mirrorHint || '未能自动获取镜像列表，请到上游面板确认镜像 ID 后手填' }}
            </template>
          </div>
        </t-form-item>
        <t-form-item label="登录端口（可选）">
          <t-input-number v-model="reinstall.port" :min="0" :max="65535" theme="column" placeholder="留空保持默认" />
          <div class="cell-sub">
            重装后 SSH/RDP 的监听端口。留空或 0 = 保持平台默认（一般 22/3389）。 填了就按你填的改 ——
            客户改过端口又要重装的场景用它，避免重装后端口被重置回默认。
          </div>
        </t-form-item>
        <t-form-item v-if="reinstallSupportsDiskSize" label="系统盘大小 GB（可选）">
          <t-input-number v-model="reinstall.system_disk_size" :min="0" theme="column" placeholder="留空保持当前" />
          <div class="cell-sub">
            重装同时调整系统盘容量。留空 = 不改容量。仅魔方云（自营平台）支持该参数；
            财务型上游的接口没有这一项，故此处不展示。
          </div>
        </t-form-item>
        <t-form-item label="同时格式化数据盘">
          <t-switch v-model="reinstall.format_data_disk" />
          <div class="cell-sub">
            <b>关闭（默认）</b>：只清系统盘，数据盘上的文件保留。<br />
            <b>开启</b>：数据盘也一并清空，<b>盘上数据永久丢失且不可恢复</b>。
            仅在确认客户不需要数据盘数据、或要交给下一位客户时开启。
          </div>
        </t-form-item>
        <t-form-item label="原因（可选）">
          <t-input v-model="reinstall.reason" placeholder="写入操作流水" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="resetPwd.visible"
      header="重置实例登录密码"
      :confirm-btn="{ content: '重置密码', theme: 'danger' }"
      :confirm-disabled="!resetPwd.password || !resetPwd.agreed"
      destroy-on-close
      @confirm="handleResetPassword"
    >
      <t-alert theme="warning" class="dlg-alert">
        <template #message>
          <div>
            重置密码由上游执行，<b>多数平台会强制重启实例</b>（部分平台第二次重启才生效），重启期间业务中断约 1–3 分钟。
          </div>
          <div class="cell-sub">系统盘与数据盘的数据不受影响，仅更换登录密码。重启前请确认客户已保存业务数据。</div>
        </template>
      </t-alert>
      <t-form label-align="top">
        <t-form-item label="新密码">
          <div class="pwd-input">
            <t-input v-model="resetPwd.password" type="password" placeholder="建议 12 位以上，含大小写与数字" />
            <t-button variant="outline" @click="fillRandomPassword">随机生成</t-button>
          </div>
        </t-form-item>
        <t-form-item>
          <t-checkbox v-model="resetPwd.agreed">已知悉：重置密码可能导致实例强制重启，业务会短暂中断</t-checkbox>
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="rescue.visible"
      header="进入救援系统"
      :confirm-btn="{ content: '进入救援' }"
      :confirm-disabled="!rescue.temp_password"
      destroy-on-close
      @confirm="handleRescue"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="进入救援后系统变为临时系统，处理完请务必点「退出救援」回到原系统。"
      />
      <t-form label-align="top">
        <t-form-item label="救援系统类型">
          <t-radio-group v-model="rescue.system" variant="default-filled">
            <t-radio-button :value="1">类型 1</t-radio-button>
            <t-radio-button :value="2">类型 2</t-radio-button>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="临时密码">
          <t-input v-model="rescue.temp_password" type="password" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="snap.visible"
      header="快照与备份"
      width="680px"
      :footer="false"
      destroy-on-close
      @close="snap.visible = false"
    >
      <t-space class="dlg-toolbar">
        <t-input v-model="snap.name" placeholder="快照名称（留空自动生成）" class="dlg-grow" />
        <t-select v-model="snap.type" :options="snapTypeOptions" class="dlg-select" />
        <t-button theme="primary" :loading="snap.loading" @click="handleCreateSnapshot">创建</t-button>
        <t-button variant="outline" :loading="snap.loading" @click="loadSnapshots(false)">刷新</t-button>
      </t-space>
      <t-table
        row-key="id"
        :data="snap.rows"
        :columns="snapColumns"
        :loading="snap.loading"
        size="small"
        cell-empty-content="暂无快照"
      >
        <template #snap_action="{ row }">
          <t-space size="small">
            <t-link theme="warning" hover="color" @click="openRestoreSnapshot(row)">恢复</t-link>
            <t-link theme="danger" hover="color" @click="handleDeleteSnapshot(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
    </t-dialog>

    <t-dialog
      v-model:visible="snap.restoreVisible"
      header="用快照恢复实例"
      :confirm-btn="{ content: '确认恢复', theme: 'danger' }"
      :confirm-disabled="snap.restoreMark.trim() !== (instanceData?.instance_id || '')"
      destroy-on-close
      @confirm="handleRestoreSnapshot"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="恢复会覆盖当前系统盘，盘上数据将回到快照时刻。请输入实例标识确认。"
      />
      <p class="dlg-confirm-tip">
        请输入实例标识 <code>{{ instanceData?.instance_id }}</code>
      </p>
      <t-input v-model="snap.restoreMark" placeholder="实例标识" />
    </t-dialog>

    <t-dialog
      v-model:visible="hw.visible"
      header="硬件变更"
      :footer="false"
      destroy-on-close
      @close="hw.visible = false"
    >
      <t-form label-align="top">
        <t-form-item label="带宽（Mbps · 0 表示不改该方向）">
          <t-space>
            <t-input-number v-model="hw.in_bw" :min="0" theme="column" />
            <span class="dlg-unit">流入</span>
            <t-input-number v-model="hw.out_bw" :min="0" theme="column" />
            <span class="dlg-unit">流出</span>
            <t-button size="small" theme="primary" :disabled="!hw.in_bw && !hw.out_bw" @click="handleSetBandwidth">
              提交
            </t-button>
          </t-space>
        </t-form-item>
        <t-form-item label="增加 IP">
          <t-space>
            <t-radio-group v-model="hw.ip_version" variant="default-filled">
              <t-radio-button :value="4">IPv4</t-radio-button>
              <t-radio-button :value="6">IPv6</t-radio-button>
            </t-radio-group>
            <t-input-number v-model="hw.ip_num" :min="1" theme="column" />
            <t-input v-model="hw.ip_group" placeholder="IP 分组 ID（可选）" />
            <t-button size="small" theme="primary" :disabled="!hw.ip_num" @click="handleAddIP">提交</t-button>
          </t-space>
        </t-form-item>
        <t-form-item label="挂载数据盘（GB）">
          <t-space>
            <t-input-number v-model="hw.disk_size" :min="1" theme="column" />
            <t-input v-model="hw.store" placeholder="存储 ID（可选）" />
            <t-button size="small" theme="primary" :disabled="!hw.disk_size" @click="handleAttachDisk">提交</t-button>
          </t-space>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 重装后平台签发的新凭据：只显示这一次，必须提示立即保存 -->
    <t-dialog
      v-model:visible="credential.visible"
      header="重装已受理 · 请立即保存新凭据"
      :confirm-btn="{ content: '我已保存' }"
      :close-btn="false"
      destroy-on-close
      @confirm="credential.visible = false"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="平台新签发的登录凭据只显示这一次，本系统不保存；关闭后只能去平台面板重置。"
      />
      <t-alert
        v-if="!credential.password"
        theme="info"
        class="dlg-alert"
        message="该服务商（如魔方财务）重装为异步任务且不回传新凭据：新密码由平台在重装完成后写入主机，请稍后到平台面板或「重置密码」获取。"
      />
      <t-descriptions :column="1" bordered size="small">
        <t-descriptions-item label="用户名">{{ credential.username || '—' }}</t-descriptions-item>
        <t-descriptions-item label="新密码">
          <code class="cell-strong">{{
            credential.password || '（平台未返回，请改用「重置密码」或到平台面板查看）'
          }}</code>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  ChatIcon,
  CpuIcon,
  DesktopIcon,
  EditIcon,
  LinkIcon,
  RefreshIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import OsIcon from '@/components/OsIcon.vue'
import { osDisplayName } from './os-icons'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  addInstanceIP,
  adjustInstanceBilling,
  attachInstanceDisk,
  createInstanceSnapshot,
  deleteInstanceSnapshot,
  destroyInstance,
  exitRescueInstance,
  getInstanceBillingContext,
  getInstanceDetail,
  getInstanceOperations,
  getInstanceRelated,
  getInstanceReinstallImages,
  getInstanceRebindCandidates,
  getInstanceSnapshots,
  getInstanceVNC,
  powerInstance,
  rebindInstanceProduct,
  reinstallInstance,
  relinkInstanceUpstream,
  resetInstancePassword,
  resizeInstance,
  rescueInstance,
  restoreInstanceSnapshot,
  retryInstanceProvision,
  revealInstanceCredential,
  suspendInstance,
  syncInstance,
  unsuspendInstance,
  updateInstanceBandwidth,
  updateInstanceRemark,
} from '@/api/instance'
import { renewInstance } from '@/api/lifecycle'
import {
  expireStateLabel,
  formatMemory,
  formatPrice,
  formatTime,
  instanceStatusLabel,
  lifecycleStageLabel,
  operationActionLabel,
  powerStateView,
  serviceStateView,
  type InstanceTransitionView,
  operationResultLabel,
  operatorTypeLabel,
  powerActionLabel,
  powerActionTips,
} from '@/pages/instances/constants'
import type {
  InstanceAdjustContext,
  InstanceDetail,
  InstanceRebindProductCandidates,
  InstanceRebindProductResult,
  InstanceRelatedResponse,
  InstanceRelinkResult,
  InstanceSnapshotInfo,
  OperationItem,
  RelatedOrder,
  RelatedRenewal,
  RelatedTicket,
} from '@/types/interface'
import { usePermission } from '@/composables/usePermission'
import { copyText } from '@/utils/clipboard'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const router = useRouter()

const instanceId = Number(route.params.id)
const instanceData = ref<InstanceDetail | null>(null)
const loading = ref(false)
const syncing = ref(false)
const actioning = ref(false)
const activeTab = ref('overview')

const { has: hasPerm } = usePermission()

const operations = ref<OperationItem[]>([])
const opLoading = ref(false)
const opPagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const related = reactive<InstanceRelatedResponse>({ orders: [], renewals: [], tickets: [] })
const relatedLoading = ref(false)

const operationColumns: PrimaryTableCol<OperationItem>[] = [
  { colKey: 'created_at', title: '时间', width: 170 },
  { colKey: 'action', title: '动作', width: 110 },
  { colKey: 'result', title: '结果', width: 90 },
  { colKey: 'operator', title: '操作人', width: 140 },
  { colKey: 'status_change', title: '状态变化', minWidth: 200 },
  { colKey: 'detail', title: '详情', minWidth: 180 },
]

const relatedOrderColumns: PrimaryTableCol<RelatedOrder>[] = [
  { colKey: 'order_no', title: '订单号', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'product_name', title: '产品名称', minWidth: 160 },
  { colKey: 'total_amount', title: '应付', width: 110 },
  { colKey: 'paid_amount', title: '实付', width: 110 },
  { colKey: 'pay_method', title: '支付方式', width: 110 },
  { colKey: 'created_at', title: '下单时间', width: 170 },
]

const relatedRenewalColumns: PrimaryTableCol<RelatedRenewal>[] = [
  { colKey: 'renewal_no', title: '续费单号', minWidth: 180 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'period_count', title: '周期', width: 80, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'expire_before', title: '原到期', width: 170 },
  { colKey: 'expire_after', title: '新到期', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

const relatedTicketColumns: PrimaryTableCol<RelatedTicket>[] = [
  { colKey: 'ticket_no', title: '工单号', minWidth: 170 },
  { colKey: 'title', title: '标题', minWidth: 200 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

// ===== 展示辅助 =====

const billingCycleOptions = [
  { value: 'hourly', label: '按小时' },
  { value: 'daily', label: '按日' },
  { value: 'monthly', label: '月付' },
  { value: 'quarterly', label: '季付' },
  { value: 'semiannually', label: '半年付' },
  { value: 'annually', label: '年付' },
  { value: 'biennially', label: '两年' },
  { value: 'triennially', label: '三年' },
  { value: 'onetime', label: '一次性' },
]

function cycleLabel(cycle: string | undefined): string {
  if (!cycle) return '—'
  const found = billingCycleOptions.find((item) => item.value === cycle)
  return found ? found.label : cycle
}

const billingModeLabel = computed(() => cycleLabel(instanceData.value?.billing_mode))

const isOnetime = computed(() => instanceData.value?.billing_mode === 'onetime')

// 头部状态标签：**两个维度分开显示**（本轮修复的核心）。
//
// 此前是一个合成标签，把"关机"与"到期暂停"压成同一句「已暂停」—— 因为上游把
// suspended 也映射成 stopped，两者无法区分（用户实测反馈）。
// 现在头部并排两个标签，各自回答一个问题：
//   · 电源维度 headlinePower  ：这台机器通电了吗（运行中/已关机/重启中…）
//   · 服务维度 headlineService：我们卖的服务还在有效期内吗（服务中/已过期/到期暂停…）
// 组合起来才是完整处境，例如「已关机 · 服务中」= 客户自己关的机，账期正常。
const headlinePower = computed(() =>
  powerStateView({
    status: instanceData.value?.status || '',
    power_state: instanceData.value?.power_state,
    lifecycle_stage: instanceData.value?.lifecycle_stage,
    pending: isPending.value,
    transition: instanceData.value?.transition,
  }),
)

const headlineService = computed(() =>
  serviceStateView({
    lifecycle_stage: instanceData.value?.lifecycle_stage,
    expire_state: instanceData.value?.expire_state,
    service_state: instanceData.value?.service_state,
    days_left: instanceData.value?.days_left,
  }),
)

/** 过渡态（上游正在执行的动作）：驱动头部提示条与操作禁用。 */
const transition = computed(() => instanceData.value?.transition || null)
const transitionActive = computed(() => !!transition.value && !transition.value.overdue)
const transitionOverdue = computed(() => !!transition.value?.overdue)

/**
 * 已执行时长的实时文案（"已等待 42 秒"）。
 *
 * 为什么需要每秒跳动：动画只证明"页面在动"，不证明"指令在跑"。一个每秒递增的
 * 数字是唯一能让运营确信"系统确实在推进这件事"的证据（这也是上游不回报进度时
 * 我们唯一能给的确定信息）。没有它，等 30 秒和卡死 30 秒看起来一样，
 * 用户就会去点第二次（正是要解决的问题）。
 */
const transitionTick = ref(0)
let transitionTimer: number | undefined

const transitionElapsedText = computed(() => {
  void transitionTick.value // 依赖 tick，确保每秒重算
  const started = transition.value?.started_at
  if (!started) return '请稍候'
  const start = new Date(started).getTime()
  if (Number.isNaN(start)) return '请稍候'
  const sec = Math.max(0, Math.floor((Date.now() - start) / 1000))
  if (sec < 60) return `已等待 ${sec} 秒`
  const min = Math.floor(sec / 60)
  return `已等待 ${min} 分 ${sec % 60} 秒`
})

// 只在过渡进行中有意义，结束后必须停掉，否则每秒重渲染整个详情页。
watch(transitionActive, (active) => {
  if (active && transitionTimer === undefined) {
    transitionTimer = window.setInterval(() => {
      transitionTick.value += 1
    }, 1000)
  } else if (!active && transitionTimer !== undefined) {
    window.clearInterval(transitionTimer)
    transitionTimer = undefined
  }
})

onBeforeUnmount(() => {
  if (transitionTimer !== undefined) {
    window.clearInterval(transitionTimer)
    transitionTimer = undefined
  }
})

/**
 * 生命周期阶段的补充说明（只在非 active 阶段给）。
 *
 * 主信息区的两个状态标签已经表达了"机器通电吗/账期有效吗"，但阶段名本身
 * （宽限期/已暂停/已销毁）不解释"这意味着什么"。概览的「上游状态」项里
 * 带上这一句，运营不用去查文档就知道下一步该做什么。
 */
const lifecycleNotice = computed(() => {
  switch (instanceData.value?.lifecycle_stage) {
    case 'grace':
      return '已过期但仍在宽限期内，逾期将暂停服务'
    case 'suspended':
      return '已暂停服务：续费或手动恢复后可用'
    case 'destroyed':
      return '已终止，不可恢复'
    default:
      return ''
  }
})

/**
 * 是否转售链路（后端归一后的判据）。
 *
 * 为什么统一用它而不是比 source_mode：历史行可能为空，前端自己判等容易漏；
 * 后端已把空值按更安全的语义归一（有上游镜像即按对接处理）。本页所有
 * 「上游实例号 / 重新关联 / 更换对接商品 / 上游各周期价」的措辞与入口
 * 都以它为准 —— 自营实例（resale=false）不得出现这些内容
 *（用户实测反馈："虽然是我自己的，但它这里会显示上游实例号，这个一看就是有问题"）。
 */
const resale = computed(() => instanceData.value?.resale === true)

const sourceModeLabel = computed(() => (resale.value ? '对接上游' : '自营'))

const sourceModeTheme = computed(() => (resale.value ? 'warning' : 'success'))

const sourceModeHint = computed(() =>
  resale.value
    ? '商品对接上游转售，实例由上游开通与控制'
    : '自营商品，实例由本平台开通与计费',
)

function productSourceModeLabel(mode: string): string {
  if (mode === 'upstream') return '对接上游（转售）'
  if (mode === 'self') return '自营（平台资源池）'
  return mode || '—'
}

const providerTypeLabels: Record<string, string> = {
  dcimcloud: '魔方云',
  zjmf: '魔方财务',
  manual: '人工',
}

function providerTypeLabel(type: string): string {
  return providerTypeLabels[type] || type || '—'
}

const orderStatusLabels: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  provisioning: '开通中',
  active: '已完成',
  cancelled: '已取消',
  refunded: '已退款',
  failed: '失败',
}

function orderStatusLabel(status: string): string {
  return orderStatusLabels[status] || status || '—'
}

function markupLabel(product: { upstream_markup_type: string; upstream_markup_value: number }): string {
  if (!product.upstream_markup_type || product.upstream_markup_type === 'none') return '不调价'
  if (product.upstream_markup_type === 'percent') return `加价 ${product.upstream_markup_value}%`
  if (product.upstream_markup_type === 'fixed') return `加价 ¥${formatPrice(product.upstream_markup_value)}`
  return product.upstream_markup_type
}

const configs = computed(() => instanceData.value?.configs || [])

const configSourceLabels: Record<string, string> = {
  instance: '实例实际情况',
  order: '客户选配',
  product: '商品默认',
  upstream: '上游固定',
}

function configSourceLabel(source: string): string {
  return configSourceLabels[source] || source || '—'
}

function configSourceTheme(source: string): string {
  switch (source) {
    case 'instance':
      return 'success'
    case 'order':
      return 'primary'
    case 'upstream':
      return 'warning'
    default:
      return 'default'
  }
}

const upstreamCycleRows = computed(() => {
  const up = instanceData.value?.upstream
  if (!up) return []
  const cycles = new Set([...Object.keys(up.list_cycles || {}), ...Object.keys(up.cost_cycles || {})])
  return Array.from(cycles)
    .filter((cycle) => (up.list_cycles?.[cycle] || 0) > 0 || (up.cost_cycles?.[cycle] || 0) > 0)
    .sort((a, b) => cycleSort(a) - cycleSort(b))
    .map((cycle) => ({
      cycle,
      list: up.list_cycles?.[cycle] || 0,
      cost: up.cost_cycles?.[cycle] || 0,
    }))
})

function cycleSort(cycle: string): number {
  const idx = billingCycleOptions.findIndex((item) => item.value === cycle)
  return idx === -1 ? 999 : idx
}

const upstreamRenewText = computed(() => {
  const amount = instanceData.value?.order?.upstream_renew_amount ?? 0
  const cycle = instanceData.value?.order?.upstream_renew_cycle
  if (!amount) return '未取到上游周期价'
  return `¥${formatPrice(amount)}${cycle ? ` / ${cycleLabel(cycle)}` : ''}`
})

// 账期条的到期着色：只在需要提醒时给颜色（临期橙、过期红），正常不外露颜色。
const expireGlanceClass = computed(() => {
  if (instanceData.value?.expire_state === 'expired') return 'hero-billing--danger'
  if (instanceData.value?.expire_state === 'expiring') return 'hero-billing--warning'
  return ''
})

// ---- 商品与对接（去重后的对照口径）----

/**
 * 上游是否改过名：对接商品的 products.name 是从资源镜像生成的，默认与上游同名。
 * 只有运营改过名才会不同 —— 这时才值得多显示一行"上游原名"，否则就是重复。
 */
const upstreamRenamed = computed(() => {
  const productName = instanceData.value?.product?.name?.trim()
  const upstreamName = instanceData.value?.upstream?.name?.trim()
  if (!productName || !upstreamName) return false
  return productName !== upstreamName
})

/** 上游采购价与本地记录不一致（上游调价但本地未同步）：续费前要核对。 */
const upstreamCostDiffers = computed(() => {
  const upstream = instanceData.value?.upstream
  const localCost = instanceData.value?.product?.cost_price ?? 0
  if (!upstream || localCost <= 0) return false
  return Math.abs(upstream.cost_price - localCost) > 0.009
})

/**
 * 毛利率：给运营一个"能不能让利"的直观数字。
 * 成本为 0（虚拟商品/未记成本）时不出，避免显示成 100% 的假象。
 */
const grossMarginText = computed(() => {
  const price = instanceData.value?.product?.price ?? 0
  const cost = instanceData.value?.product?.cost_price ?? 0
  if (price <= 0 || cost <= 0 || cost >= price) return ''
  return `毛利 ${(((price - cost) / price) * 100).toFixed(1)}%`
})

function powerStatusText(status: string): string {
  if (status === 'on') return '开机'
  if (status === 'off') return '关机'
  return '未知'
}

// ===== NAT 机型访问地址（doc122）=====
// NAT 机型没有独立公网 IP，连接信息来自节点转发地址；只显示 public_ip 会
// 让运营以为机器没配好，实际是机型本身就没有独立 IP。
/** 是否 NAT 机型：以后端结论为准（依据 raw_data 机型字段，不靠 public_ip 是否为空猜）。 */
const isNatInstance = computed(() => instanceData.value?.nat_access?.is_nat === true)

/** NAT 访问信息（模板里不能直接用 instanceData.nat_access：instanceData 可能为 null）。 */
const natAccess = computed(() => instanceData.value?.nat_access ?? null)

/** NAT 主地址：优先端口转发，其次建站域名；都没有时返回空串（界面提示暂无）。 */
const natPrimaryAddress = computed(() => {
  const nat = natAccess.value
  if (!nat) return ''
  if (nat.forwards?.length) return nat.forwards[0]
  if (nat.domains?.length) return nat.domains[0]
  return ''
})

/** NAT 读取失败提示（与"没有转发规则"区分：前者可重试，后者是配置状态）。 */
const natError = computed(() => natAccess.value?.error || '')

function expireClass(row: InstanceDetail): string {
  if (row.expire_state === 'expired') return 'expire-danger'
  if (row.expire_state === 'expiring') return 'expire-warning'
  return ''
}

function capabilityTitle(supported: boolean | undefined): string {
  return supported ? '' : '该服务商不支持此操作'
}

function operationResultTheme(result: string): string {
  if (result === 'success') return 'success'
  if (result === 'skipped') return 'warning'
  return 'danger'
}

const consoleDisabled = computed(
  () => !instanceData.value?.capabilities?.console || instanceData.value?.status !== 'running',
)

const consoleTitle = computed(() => {
  if (!instanceData.value?.capabilities?.console) return '该服务商不支持此操作'
  if (instanceData.value?.status !== 'running') return '仅运行中的实例可打开控制台'
  return ''
})

function powerDisabled(action: string): boolean {
  if (!instanceData.value?.capabilities?.power) return true
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on'
  if (action === 'off') return ps === 'off'
  return ps !== 'on'
}

function powerTitle(action: string): string {
  if (!instanceData.value?.capabilities?.power) return '该服务商不支持此操作'
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on' ? '实例已处于运行状态' : ''
  if (action === 'off') return ps === 'off' ? '实例已处于关机状态' : ''
  return ps !== 'on' ? '仅运行中的实例可重启' : ''
}

// ===== 数据加载 =====

async function loadDetail() {
  loading.value = true
  try {
    instanceData.value = await getInstanceDetail(instanceId, false)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例详情失败')
  } finally {
    loading.value = false
  }
}

async function loadOperations() {
  opLoading.value = true
  try {
    const data = await getInstanceOperations(instanceId, {
      page: opPagination.current,
      page_size: opPagination.pageSize,
    })
    operations.value = data.items
    opPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载操作记录失败')
  } finally {
    opLoading.value = false
  }
}

async function loadRelated() {
  relatedLoading.value = true
  try {
    const data = await getInstanceRelated(instanceId)
    related.orders = data.orders
    related.renewals = data.renewals
    related.tickets = data.tickets
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载关联记录失败')
  } finally {
    relatedLoading.value = false
  }
}

function reloadAll() {
  loadDetail()
  loadOperations()
}

function handleOpPageChange(pageInfo: PageInfo) {
  opPagination.current = pageInfo.current
  opPagination.pageSize = pageInfo.pageSize
  loadOperations()
}

function goBack() {
  router.back()
}

function openOrder(row: RelatedOrder) {
  router.push(`/orders/detail/${row.id}`)
}

function openOrderById(id: number) {
  if (id > 0) router.push(`/orders/detail/${id}`)
}

function openProductById(id: number) {
  if (id > 0) router.push(`/product/products/${id}`)
}

function openTicket(row: RelatedTicket) {
  router.push(`/tickets/detail/${row.id}`)
}

// ===== 操作分组下拉 =====

const powerOptions = computed(() => {
  const opts: { content: string; value: string; disabled?: boolean; theme?: string }[] = [
    { content: '开机', value: 'on', disabled: powerDisabled('on') },
    { content: '关机', value: 'off', disabled: powerDisabled('off') },
    { content: '重启', value: 'reboot', disabled: powerDisabled('reboot') },
    { content: '强制关机（断电）', value: 'hard_off', disabled: powerDisabled('hard_off'), theme: 'error' },
    { content: '强制重启（断电）', value: 'hard_reboot', disabled: powerDisabled('hard_reboot'), theme: 'error' },
  ]
  return opts
})

// 待开通（尚未在上游开通的占位行）：只开放「重新开通」。
const isPending = computed(() => !!instanceData.value && !instanceData.value.provider_instance_id)
const retrying = ref(false)

// 开通失败横幅：manual=上游已明确拒绝（如余额不足），重试无效需人工；failed=系统自动重试中。
const provisionAlertTitle = computed(() => {
  if (instanceData.value?.provision_status === 'manual') return '开通失败，需人工处理'
  if (instanceData.value?.provision_status === 'failed') return '开通失败，系统自动重试中'
  return '实例待开通'
})
const provisionAlertMessage = computed(() => {
  const reason = instanceData.value?.provision_error
  if (reason) {
    return `上游返回：${reason}。处理后可点右上角「重新开通」—— 重试会复用已生成的上游账单，不会重复下单。`
  }
  return '该实例尚未在上游开通（订单未支付或开通中）。未支付订单超出支付有效期会自动关单并删除本实例；已支付但开通失败的可在修复后点「重新开通」。'
})

async function handleRetryProvision() {
  if (!instanceData.value) return
  retrying.value = true
  try {
    await retryInstanceProvision(instanceData.value.id)
    MessagePlugin.success('已重新提交开通，稍后刷新查看状态')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重新开通失败')
  } finally {
    retrying.value = false
  }
}

// 电源组在过渡态期间禁用：上游指令是异步的，执行窗口内再点会堆指令
// （用户实测反馈"看不到状态就反复点重启"）。超时后放行，允许重试。
const powerGroupDisabled = computed(
  () => isPending.value || !instanceData.value?.capabilities?.power || transitionActive.value,
)

function onPowerPick(data: { value: string; disabled?: boolean }) {
  if (data.disabled) {
    MessagePlugin.warning(powerTitle(data.value) || '该操作当前不可用')
    return
  }
  handlePower(data.value)
}

// 维护类操作（平台接口实测存在）。
// 重装/重置密码**不在这里**：主信息区已经把这两个入口内联在「操作系统」行与
// 「登录凭据」格里（用户实测反馈按钮重复）。下拉只保留没有就地入口的项。
const maintenanceOptions = computed(() => {
  const caps = instanceData.value?.capabilities
  const opts: { content: string; value: string; disabled?: boolean }[] = []
  if (hasPerm('instance:rescue')) {
    opts.push({ content: '进入救援系统', value: 'rescue', disabled: !caps?.rescue })
    opts.push({ content: '退出救援系统', value: 'exitrescue', disabled: !caps?.rescue })
  }
  if (hasPerm('instance:snapshot')) {
    opts.push({ content: '快照与备份', value: 'snapshot', disabled: !caps?.snapshot })
  }
  if (hasPerm('instance:hardware')) {
    opts.push({
      content: '硬件变更',
      value: 'hardware',
      disabled: !caps?.bandwidth && !caps?.add_ip && !caps?.attach_disk,
    })
  }
  return opts
})

// 维护类（重装/重置密码/救援）同样受过渡态约束：这些动作本身就会写过渡态，
// 且与进行中的动作互斥（重装到一半再点重置密码只会让状态更乱）。
const maintenanceDisabled = computed(
  () => isPending.value || maintenanceOptions.value.length === 0 || transitionActive.value,
)

/**
 * 打开重装弹窗。抽成函数而不是留在下拉的 case 里：主信息区的「操作系统」行
 * 也挂了一个「重装」按钮（换系统是高频操作，就近给入口），两处必须同一套
 * 重置逻辑，否则从其中一个入口进来会带着上次的残留值。
 */
function openReinstallDialog() {
  reinstall.os = ''
  reinstall.port = 0
  reinstall.system_disk_size = 0
  reinstall.format_data_disk = false
  reinstall.reason = ''
  reinstall.visible = true
  loadMirrorOptions()
}

function onMaintenancePick(data: { value: string; disabled?: boolean }) {
  if (data.disabled) {
    MessagePlugin.warning('当前服务商不支持该操作')
    return
  }
  switch (data.value) {
    case 'reinstall':
      openReinstallDialog()
      break
    case 'resetpwd':
      openResetPasswordDialog()
      break
    case 'rescue':
      rescue.system = 1
      rescue.temp_password = ''
      rescue.visible = true
      break
    case 'exitrescue':
      handleExitRescue()
      break
    case 'snapshot':
      snap.name = ''
      snap.type = 'snap'
      snap.visible = true
      loadSnapshots(true)
      break
    case 'hardware':
      hw.visible = true
      break
  }
}

// 账期与凭证分组。
// 「查看登录密码」不在这里：凭据格已有内联「查看密码」链接（用户实测反馈重复）。
const billingOptions = computed(() => {
  const opts: { content: string; value: string; disabled?: boolean }[] = []
  if (hasPerm('instance:billing')) {
    opts.push({ content: '调整账期/付款周期', value: 'adjust' })
  }
  if (hasPerm('instance:action')) {
    opts.push({ content: '编辑备注', value: 'remark' })
  }
  return opts
})

const billingDisabled = computed(() => billingOptions.value.length === 0)

function onBillingPick(data: { value: string; disabled?: boolean }) {
  if (data.disabled) {
    MessagePlugin.warning('该操作当前不可用')
    return
  }
  switch (data.value) {
    case 'adjust':
      openAdjustDialog()
      break
    case 'remark':
      openRemarkDialog()
      break
  }
}

// 危险操作分组
const dangerOptions = computed(() => {
  const opts: { content: string; value: string; disabled?: boolean; theme?: string }[] = []
  if (hasPerm('instance:action') && instanceData.value?.lifecycle_stage === 'suspended') {
    opts.push({ content: '恢复服务', value: 'unsuspend' })
  } else if (hasPerm('instance:action')) {
    opts.push({ content: '暂停服务', value: 'suspend', disabled: !instanceData.value?.capabilities?.suspend })
  }
  if (hasPerm('instance:destroy')) {
    opts.push({
      content: '销毁实例',
      value: 'destroy',
      disabled: !instanceData.value?.capabilities?.destroy,
      theme: 'error',
    })
  }
  return opts
})

const dangerDisabled = computed(() => isPending.value || dangerOptions.value.length === 0)

function onDangerPick(data: { value: string; disabled?: boolean }) {
  if (data.disabled) {
    MessagePlugin.warning('当前服务商不支持该操作')
    return
  }
  switch (data.value) {
    case 'suspend':
      handleSuspend()
      break
    case 'unsuspend':
      handleUnsuspend()
      break
    case 'destroy':
      openDestroyDialog()
      break
  }
}

// ===== 电源 =====

function handlePower(action: string) {
  if (action === 'on') {
    doPower(action)
    return
  }
  const name = instanceData.value?.name || instanceData.value?.instance_id || ''
  const label = powerActionLabel(action)
  const dialog = DialogPlugin.confirm({
    header: `${label}实例`,
    body: `确认对实例「${name}」执行${label}？${powerActionTips[action] || ''}`,
    confirmBtn: { content: `确认${label}`, theme: action === 'reboot' ? 'warning' : 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      await doPower(action)
    },
    onClose: () => dialog.destroy(),
  })
}

async function doPower(action: string) {
  try {
    await powerInstance(instanceId, action)
    MessagePlugin.success(`${powerActionLabel(action)}指令已发送`)
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${powerActionLabel(action)}失败`)
  }
}

async function handleSync() {
  syncing.value = true
  try {
    await syncInstance(instanceId)
    MessagePlugin.success('实例状态已同步')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncing.value = false
  }
}

// —— 暂停/恢复（T5.5）——
function handleSuspend() {
  const name = instanceData.value?.name || instanceData.value?.instance_id || ''
  const dialog = DialogPlugin.confirm({
    header: '暂停实例服务',
    body: `确认暂停实例「${name}」？暂停后平台侧实例将停止服务，用户需续费或联系客服才能恢复。`,
    confirmBtn: { content: '确认暂停', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      actioning.value = true
      try {
        await suspendInstance(instanceId)
        MessagePlugin.success('实例已暂停')
        reloadAll()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '暂停失败')
      } finally {
        actioning.value = false
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function handleUnsuspend() {
  actioning.value = true
  try {
    await unsuspendInstance(instanceId)
    MessagePlugin.success('实例已恢复')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复失败')
  } finally {
    actioning.value = false
  }
}

// ===== 备注 =====

const remarkVisible = ref(false)
const remarkForm = reactive<{ remark: string }>({ remark: '' })

function openRemarkDialog() {
  remarkForm.remark = instanceData.value?.remark ?? ''
  remarkVisible.value = true
}

async function handleSaveRemark() {
  try {
    await updateInstanceRemark(instanceId, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

// ===== 账期调整（本地口径）=====

const adjust = reactive<{
  visible: boolean
  expire_at: string
  billing_mode: string
  renewal_price: number | null
  force: boolean
  reason: string
  loading: boolean
}>({ visible: false, expire_at: '', billing_mode: '', renewal_price: null, force: false, reason: '', loading: false })

const adjustConfirmDisabled = computed(
  () => !adjust.expire_at && !adjust.billing_mode && !adjustRenewalPriceDirty.value,
)

// 绑定价是否被改动过。预填了当前值，所以"提交时带上 renewal_price"不能只看非空 ——
// 否则每次调账期都会把未改动的绑定价重写一遍（流水里凭空多一条改价记录）。
const adjustRenewalPriceDirty = computed(() => {
  const current = instanceData.value?.renewal_price ?? null
  const next = adjust.renewal_price
  if (current === null && next === null) return false
  if (current === null || next === null) return true
  return Math.abs(current - next) > 0.001
})

// 账期调整上下文：可选周期（上游能力）+ 各周期上游价/我方价 + 当前绑定价。
// 由后端算而非前端硬编码 —— 周期列表必须反映上游真实支持范围。
const adjustCtx = ref<InstanceAdjustContext | null>(null)
const adjustCtxLoading = ref(false)

const adjustCycleOptions = computed(() =>
  (adjustCtx.value?.cycles || []).map((c) => ({
    value: c.cycle,
    label: c.name,
    // 有上游采购价时在选项里带出来，省一次展开比价
    content: c.upstream_cost > 0 ? `${c.name}（上游采购 ¥${c.upstream_cost.toFixed(2)}）` : c.name,
  })),
)

const selectedCycleRow = computed(
  () => (adjustCtx.value?.cycles || []).find((c) => c.cycle === adjust.billing_mode) || null,
)

// 选中周期的毛利提示（我方售价 vs 上游采购价）：差值为负是明显亏损，需警示。
const selectedCycleMarginText = computed(() => {
  const row = selectedCycleRow.value
  if (!row || !row.has_product_price || row.upstream_cost <= 0) return ''
  const diff = row.sale - row.upstream_cost
  const pct = row.upstream_cost > 0 ? (diff / row.upstream_cost) * 100 : 0
  if (diff < 0)
    return `⚠ 按此周期售价 ¥${row.sale.toFixed(2)} 低于上游采购价 ¥${row.upstream_cost.toFixed(2)}，每期亏 ¥${Math.abs(diff).toFixed(2)}`
  return `毛利 ¥${diff.toFixed(2)}（${pct.toFixed(1)}%）`
})

const currentRenewalPriceText = computed(() => {
  const p = adjustCtx.value?.current_renewal_price
  if (p === null || p === undefined) return '未绑定（按商品实时算价）'
  return `¥${p.toFixed(2)} / ${cycleLabel(instanceData.value?.billing_mode)}`
})

// 按上游采购价填入续费价：运营常见诉求是"按成本持平/微利定价"，
// 手输容易输错，给一键填入（仍可手改）。
function fillRenewalFromUpstream() {
  const row = selectedCycleRow.value
  if (!row || row.upstream_cost <= 0) return
  adjust.renewal_price = row.upstream_cost
}

// clearAdjustRenewalPrice 清空输入即"取消绑定"：提交时脏检查判定为"从有到无"，
// 会带上 -1（后端约定：负值 = 清除绑定）。
function clearAdjustRenewalPrice() {
  adjust.renewal_price = null
}

// 切换周期时若已填续费价，提示它对应的是新周期的承诺价（不自动改数值，避免误改）。
function onAdjustCycleChange() {
  const row = selectedCycleRow.value
  if (row && adjustCtx.value && row.cycle !== adjustCtx.value.current_cycle) {
    // 仅提示，不改值：绑定价是运营刻意钉的，自动改写会变成"系统偷偷改价"。
    MessagePlugin.info(`已切到${row.name}：续费价将按该周期生效，请确认下方绑定价`)
  }
}

function openAdjustDialog() {
  adjust.expire_at = ''
  adjust.billing_mode = instanceData.value?.billing_mode ?? ''
  // 预填当前绑定价（详情已下发）：即使上下文接口稍后才回来，运营也能立刻看到现值。
  adjust.renewal_price = instanceData.value?.renewal_price ?? null
  adjust.force = false
  adjust.reason = ''
  adjust.visible = true
  void loadAdjustContext()
}

async function loadAdjustContext() {
  adjustCtxLoading.value = true
  try {
    adjustCtx.value = await getInstanceBillingContext(instanceId)
  } catch (error) {
    // 上下文读失败不清空已填内容：对话框仍可提交（后端会独立校验周期合法性）。
    MessagePlugin.warning((error as Error).message || '读取上游周期与价格失败，周期列表可能不完整')
  } finally {
    adjustCtxLoading.value = false
  }
}

async function handleAdjustBilling() {
  if (adjustConfirmDisabled.value) {
    MessagePlugin.warning('请至少填写到期时间、付款周期或续费价')
    return
  }
  adjust.loading = true
  try {
    const res = await adjustInstanceBilling(instanceId, {
      expire_at: adjust.expire_at || undefined,
      billing_mode: adjust.billing_mode || undefined,
      // 只在真的改过时才带：否则会把没动过的绑定价原样重写一遍（多余流水 + 覆盖他人改价）。
      renewal_price: adjustRenewalPriceDirty.value ? (adjust.renewal_price ?? -1) : undefined,
      force: adjust.force || undefined,
      reason: adjust.reason || undefined,
    })
    instanceData.value = res
    MessagePlugin.success('账期已调整')
    adjust.visible = false
    loadOperations()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '调整账期失败')
  } finally {
    adjust.loading = false
  }
}

// ===== 按上游实例号重新关联（上游换机/换号）=====

const relink = reactive<{
  visible: boolean
  upstream_instance_id: string
  confirm_mark: string
  migrate_instance_id: boolean
  migrateDone: boolean
  reason: string
  loading: boolean
}>({
  visible: false,
  upstream_instance_id: '',
  confirm_mark: '',
  migrate_instance_id: false,
  migrateDone: false,
  reason: '',
  loading: false,
})

const relinkResult = ref<InstanceRelinkResult | null>(null)

// 只有"本地号本来就在用上游号"时才提供同步改写选项：
// 否则 instance_id 是本地自建号（如 pending-xxx 转正后的历史值），
// 改它会让历史流水里的实例标识与新号对不上。
const relinkCanMigrateLocal = computed(
  () => !!instanceData.value?.instance_id && instanceData.value.instance_id === instanceData.value.provider_instance_id,
)

const relinkConfirmDisabled = computed(
  () => !relink.upstream_instance_id.trim() || relink.confirm_mark.trim() !== (instanceData.value?.instance_id || ''),
)

function openRelinkDialog() {
  // 自营链路没有"上游号可换"：入口已按 resale 隐藏，这里再兜一层 ——
  // 万一被别的路径触发（如快捷键/旧缓存页面），打开一个填了也只有报错的对话框
  // 比直接提示更让人困惑。
  if (!resale.value) {
    MessagePlugin.warning('该实例为自营链路，实例号由本系统管理，无需重新关联')
    return
  }
  relink.upstream_instance_id = ''
  relink.confirm_mark = ''
  relink.migrate_instance_id = false
  relink.migrateDone = false
  relink.reason = ''
  relinkResult.value = null
  relink.visible = true
}

async function handleRelink() {
  if (relinkConfirmDisabled.value) {
    MessagePlugin.warning('请填写目标上游实例号，并准确输入当前实例标识')
    return
  }
  relink.loading = true
  try {
    const res = await relinkInstanceUpstream(instanceId, {
      upstream_instance_id: relink.upstream_instance_id.trim(),
      confirm_mark: relink.confirm_mark.trim(),
      migrate_instance_id: relink.migrate_instance_id || undefined,
      reason: relink.reason || undefined,
    })
    relinkResult.value = res
    relink.migrateDone = res.instance_id_migrated
    MessagePlugin.success(`已关联到上游实例 ${res.new_provider_instance_id}`)
    // 重新拉详情：上游号/状态/IP 都已按新机器刷新，界面要跟着走。
    await loadDetail()
    loadOperations()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重新关联失败')
  } finally {
    relink.loading = false
  }
}

// ===== 更换对接商品（本地商品 / 上游镜像）=====

const rebind = reactive<{
  visible: boolean
  fields: string[]
  sell_product_id: number | null
  upstream_product_id: number | null
  renewal_price: number | null
  reason: string
  loading: boolean
}>({
  visible: false,
  fields: [],
  sell_product_id: null,
  upstream_product_id: null,
  renewal_price: null,
  reason: '',
  loading: false,
})

const rebindResult = ref<InstanceRebindProductResult | null>(null)

// 候选列表（选框数据源）。每次开弹窗重取：商品上下架/渠道会变，缓存会给出过期选项。
const rebindCandidates = ref<InstanceRebindProductCandidates | null>(null)
const rebindCandidatesLoading = ref(false)
const rebindHint = ref('')

const rebindSellOptions = computed(() =>
  (rebindCandidates.value?.sell_products || []).map((item) => ({
    // 名称 + 编码 + 价格 + 已有周期档：选之前就能比出"这个换上去续费会收多少"
    label: `#${item.id} ${item.name}${item.code ? `（${item.code}）` : ''} · ¥${formatPrice(item.price ?? 0)}${
      item.cycles?.length ? ` · ${item.cycles.map((c) => cycleLabel(c)).join('/')}` : ' · 未配周期价'
    }${item.current ? ' · 当前绑定' : ''}${item.disabled ? ` · ${item.disabled_reason || '不可选'}` : ''}`,
    value: item.id,
    disabled: item.disabled || item.current,
  })),
)

const rebindUpstreamOptions = computed(() =>
  (rebindCandidates.value?.upstream_products || []).map((item) => ({
    label: `#${item.id} ${item.name} · 上游 ¥${formatPrice(item.price ?? 0)} / 成本 ¥${formatPrice(item.cost_price ?? 0)}${
      item.current ? ' · 当前绑定' : ''
    }`,
    value: item.id,
    disabled: item.current,
  })),
)

async function loadRebindCandidates() {
  rebindCandidatesLoading.value = true
  try {
    const res = await getInstanceRebindCandidates(instanceId)
    rebindCandidates.value = res
    rebindHint.value = res?.hint || ''
  } catch (error) {
    // 候选拿不到不阻塞：运营仍可手动输入 ID（下面保留数字输入兜底）。
    rebindCandidates.value = null
    rebindHint.value = `候选列表加载失败（${(error as Error).message || '未知错误'}），可手动填写 ID`
  } finally {
    rebindCandidatesLoading.value = false
  }
}

function openRebindDialog() {
  rebind.fields = []
  rebind.sell_product_id = null
  rebind.upstream_product_id = null
  rebind.renewal_price = null
  rebind.reason = ''
  rebindResult.value = null
  rebind.visible = true
  void loadRebindCandidates()
}

async function handleRebind() {
  // 自营链路不接受上游镜像字段（后端会拒）：把选项收敛在后端口径上，
  // 避免"前端给选项 → 用户选了 → 提交才报错"这条弯路。
  if (!resale.value) {
    rebind.fields = rebind.fields.filter((f) => f !== 'upstream')
  }
  if (!rebind.fields.length) {
    MessagePlugin.warning('请至少勾选一项要更换的内容')
    return
  }
  const payload: Record<string, unknown> = {}
  if (rebind.fields.includes('sell')) {
    if (!rebind.sell_product_id) {
      MessagePlugin.warning('请选择新的本地售出商品')
      return
    }
    payload.sell_product_id = rebind.sell_product_id
  }
  if (rebind.fields.includes('upstream')) {
    if (rebind.upstream_product_id === null) {
      MessagePlugin.warning('请选择新的上游镜像')
      return
    }
    payload.upstream_product_id = rebind.upstream_product_id
  }
  if (rebind.fields.includes('renewal_price')) {
    // 空 = 清除绑定（传 -1），与后端「负值 = 清除」的约定对齐。
    payload.renewal_price = rebind.renewal_price === null ? -1 : rebind.renewal_price
  }
  payload.reason = rebind.reason || undefined

  rebind.loading = true
  try {
    const res = await rebindInstanceProduct(instanceId, payload)
    rebindResult.value = res
    MessagePlugin.success('商品绑定已更换')
    await loadDetail()
    loadOperations()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更换失败')
  } finally {
    rebind.loading = false
  }
}

// ===== 明文凭据查看 =====

const reveal = reactive<{ visible: boolean; username: string; password: string; port: string }>({
  visible: false,
  username: '',
  password: '',
  port: '',
})

async function openRevealDialog() {
  try {
    const res = await revealInstanceCredential(instanceId)
    reveal.username = res.username
    reveal.password = res.password
    reveal.port = res.port
    reveal.visible = true
    loadOperations()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '查看密码失败')
  }
}

function copyRevealPassword() {
  void copyText(reveal.password, '密码')
}

// ===== 变配 =====

const resizeVisible = ref(false)
const resizeForm = reactive<{ cpu: number; memory: number; disk: number; disk_type: string; reason: string }>({
  cpu: 0,
  memory: 0,
  disk: 0,
  disk_type: '',
  reason: '',
})

function openResizeDialog() {
  const data = instanceData.value
  if (!data) return
  resizeForm.cpu = data.cpu
  resizeForm.memory = data.memory
  resizeForm.disk = data.disk
  resizeForm.disk_type = data.disk_type
  resizeForm.reason = ''
  resizeVisible.value = true
}

async function handleResize() {
  try {
    await resizeInstance(instanceId, {
      cpu: resizeForm.cpu,
      memory: resizeForm.memory,
      disk: resizeForm.disk,
      disk_type: resizeForm.disk_type,
      reason: resizeForm.reason || undefined,
    })
    MessagePlugin.success('变配指令已提交')
    resizeVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '变配失败')
  }
}

// ===== 销毁 =====

const destroyVisible = ref(false)
const destroyForm = reactive<{ confirm_mark: string; reason: string }>({ confirm_mark: '', reason: '' })

const destroyConfirmDisabled = computed(
  () => !instanceData.value?.instance_id || destroyForm.confirm_mark.trim() !== instanceData.value.instance_id,
)

function openDestroyDialog() {
  destroyForm.confirm_mark = ''
  destroyForm.reason = ''
  destroyVisible.value = true
}

async function handleDestroy() {
  try {
    await destroyInstance(instanceId, {
      confirm_mark: destroyForm.confirm_mark.trim(),
      reason: destroyForm.reason || undefined,
    })
    MessagePlugin.success('实例已销毁')
    destroyVisible.value = false
    router.push('/instances/list')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '销毁失败')
  }
}

// ===== 续费 =====

const renewVisible = ref(false)
const renewForm = reactive<{ period_count: number; amount: number; remark: string }>({
  period_count: 1,
  amount: 0,
  remark: '',
})

function openRenewDialog() {
  renewForm.period_count = 1
  renewForm.amount = 0
  renewForm.remark = ''
  renewVisible.value = true
}

async function handleRenew() {
  try {
    await renewInstance(instanceId, {
      period_count: renewForm.period_count,
      amount: renewForm.amount || undefined,
      remark: renewForm.remark || undefined,
    })
    MessagePlugin.success('代续费成功，已延长实例到期时间')
    renewVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '代续费失败')
  }
}

// ===== 控制台 =====

const vnc = reactive({ visible: false, name: '', url: '', password: '' })
// vncBlocked 新标签页被浏览器拦截（弹窗拦截）：弹窗里据此给"手动打开"按钮。
const vncBlocked = ref(false)

// handleVnc 打开远程控制台：**新标签页**，不是弹窗内嵌（用户反馈）。
//
// 为什么新标签页更好：控制台是独立会话，弹窗一关就断；标签页可以固定、
// 可以与实例页并排、可以同时开多台。弹窗在这里只承担"交付控制台密码"。
//
// 弹窗拦截：window.open 必须在用户手势的同步链路里调用才不被拦。
// 但取地址是异步请求（await 之后就脱离手势了），所以这里先开一个空标签页占位，
// 拿到地址后再填 —— 拿不到就关掉占位页并提示。
async function handleVnc() {
  // 同步先开占位标签页（保住用户手势，避免被当广告拦截）。
  const tab = window.open('', '_blank')
  try {
    const data = await getInstanceVNC(instanceId)
    vnc.name = instanceData.value?.name || instanceData.value?.instance_id || ''
    vnc.url = data.url
    vnc.password = data.password
    vncBlocked.value = false
    if (tab && !tab.closed) {
      tab.location.href = data.url
    } else {
      // 占位页被拦截 → 记下来，弹窗里给手动打开按钮。
      vncBlocked.value = true
    }
    vnc.visible = true
  } catch (error) {
    // 取地址失败：把占位页关掉，别留一个白页给运营。
    if (tab && !tab.closed) tab.close()
    MessagePlugin.error((error as Error).message || '获取控制台地址失败')
  }
}

/** 重新打开控制台（弹窗的「再次打开」按钮）。 */
function reopenVnc() {
  if (vnc.url) {
    const tab = window.open(vnc.url, '_blank')
    vncBlocked.value = !tab || tab.closed
  }
}

/** 手动打开（占位页被拦截时的兜底）。 */
function openVnc() {
  const tab = window.open(vnc.url, '_blank')
  vncBlocked.value = !tab || tab.closed
}

// 控制台地址是非 http 时（如 vnc:// 或自定义协议），浏览器不能直接打开，
// 只能复制交给本地客户端；复制走公共 util 才有降级路径。
function copyVncUrl() {
  void copyText(vnc.url, '控制台地址')
}

function copyVncPassword() {
  void copyText(vnc.password, '控制台密码')
}

function isHttp(url: string): boolean {
  return /^https?:/i.test(url)
}

// ===== 重装 =====

const reinstall = reactive<{
  visible: boolean
  os: string
  port: number
  system_disk_size: number
  format_data_disk: boolean
  reason: string
  loading: boolean
}>({ visible: false, os: '', port: 0, system_disk_size: 0, format_data_disk: false, reason: '', loading: false })

// 可选镜像：统一由后端 /instances/:id/reinstall-images 给出。
// 来源随链路不同 —— 自营读渠道镜像目录；对接上游读**上游商品的操作系统配置子项**
// （os 值就是配置子项 ID，与开通口径一致）。此前前端只读渠道镜像目录，
// 对接链路必然拿到空列表（用户实测：重装对话框镜像为空）。
const mirrorOptions = ref<Array<{ label: string; value: string }>>([])
const mirrorSource = ref('')
const mirrorHint = ref('')
const mirrorLoading = ref(false)
// 按实例缓存加载结果：同一实例重开弹窗不重复请求，换实例必须重取。
const mirrorLoadedFor = ref<number>(0)

// reinstallSupportsDiskSize 重装时能否同时改系统盘容量。
// 实测：只有魔方云适配器的重装接口带 system_disk_size（provider.go 里 form.Set），
// 魔方财务的 ReinstallInstance 完全不读该参数 —— 对财务型上游展示这个输入框
// 只会让运营以为改了容量（实际被静默忽略）。
const reinstallSupportsDiskSize = computed(() => instanceData.value?.provider_type !== 'zjmf')

async function loadMirrorOptions() {
  const current = instanceId
  if (mirrorLoadedFor.value === current) return
  mirrorLoading.value = true
  mirrorLoadedFor.value = current
  try {
    const res = await getInstanceReinstallImages(current)
    const images = res?.images || []
    mirrorOptions.value = images
      .filter((img) => img.value)
      .map((img) => ({
        label: `${img.group ? `${img.group} · ` : ''}${img.label || img.value}${img.status ? `（${img.status}）` : ''}`,
        value: img.value,
      }))
    mirrorSource.value = res?.source || ''
    mirrorHint.value = res?.hint || ''
  } catch (error) {
    // 列表拿不到不阻塞重装：留空 + 提示，运营仍可手填镜像 ID。
    mirrorOptions.value = []
    mirrorSource.value = ''
    mirrorHint.value = (error as Error).message || ''
  } finally {
    mirrorLoading.value = false
  }
}

// 重装后平台签发的新凭据（只此一次展示）
const credential = reactive<{ visible: boolean; username: string; password: string }>({
  visible: false,
  username: '',
  password: '',
})

async function handleReinstall() {
  if (!reinstall.os) {
    MessagePlugin.warning('请填写目标镜像 ID')
    return
  }
  reinstall.loading = true
  try {
    const res = await reinstallInstance(instanceId, {
      os: reinstall.os.trim(),
      port: reinstall.port || undefined,
      system_disk_size: reinstall.system_disk_size || undefined,
      format_data_disk: reinstall.format_data_disk,
      reason: reinstall.reason || undefined,
    })
    reinstall.visible = false
    if (res?.password || res?.username) {
      credential.username = res.username || ''
      credential.password = res.password || ''
      credential.visible = true
    } else {
      MessagePlugin.success('重装指令已提交')
    }
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重装失败')
  } finally {
    reinstall.loading = false
  }
}

// ===== 重置密码 =====

const resetPwd = reactive<{ visible: boolean; password: string; agreed: boolean; loading: boolean }>({
  visible: false,
  password: '',
  agreed: false,
  loading: false,
})

// openResetPasswordDialog 打开重置密码弹窗（凭据格的内联入口与维护下拉共用）。
function openResetPasswordDialog() {
  resetPwd.password = ''
  // 同意勾选每次重开都复位：重启风险提示不能靠上一次点过就长期豁免。
  resetPwd.agreed = false
  resetPwd.visible = true
}

// fillRandomPassword 生成符合上游面板强度策略的随机密码（12 位含大小写与数字）。
// 与后端的生成器同策略：剔除 0/O/o/1/l/I 等易混淆字符 —— 这段密码要被手抄到 SSH 里。
function fillRandomPassword() {
  const lower = 'abcdefghijkmnpqrstuvwxyz'
  const upper = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
  const digit = '23456789'
  const all = lower + upper + digit
  const pick = (set: string) => set[Math.floor(Math.random() * set.length)]
  const chars = [pick(lower), pick(upper), pick(digit)]
  while (chars.length < 12) chars.push(pick(all))
  for (let i = chars.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[chars[i], chars[j]] = [chars[j], chars[i]]
  }
  resetPwd.password = chars.join('')
}

async function handleResetPassword() {
  if (!resetPwd.password) {
    MessagePlugin.warning('请输入新密码')
    return
  }
  resetPwd.loading = true
  try {
    await resetInstancePassword(instanceId, resetPwd.password)
    MessagePlugin.success('密码已重置')
    resetPwd.visible = false
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重置密码失败')
  } finally {
    resetPwd.loading = false
  }
}

// ===== 救援系统 =====

const rescue = reactive<{ visible: boolean; system: number; temp_password: string; loading: boolean }>({
  visible: false,
  system: 1,
  temp_password: '',
  loading: false,
})

async function handleRescue() {
  if (!rescue.temp_password) {
    MessagePlugin.warning('请输入临时密码')
    return
  }
  rescue.loading = true
  try {
    await rescueInstance(instanceId, { system: rescue.system, temp_password: rescue.temp_password })
    MessagePlugin.success('已发起进入救援系统')
    rescue.visible = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '进入救援系统失败')
  } finally {
    rescue.loading = false
  }
}

async function handleExitRescue() {
  const dialog = DialogPlugin.confirm({
    header: '退出救援系统',
    body: '确认退出救援系统？实例将回到原系统。',
    theme: 'warning',
    onConfirm: async () => {
      try {
        await exitRescueInstance(instanceId)
        MessagePlugin.success('已发起退出救援系统')
        reloadAll()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '退出救援系统失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

// ===== 快照与备份 =====

const snapTypeOptions = [
  { label: '快照', value: 'snap' },
  { label: '备份', value: 'backup' },
]

const snapColumns: PrimaryTableCol[] = [
  { colKey: 'name', title: '名称', minWidth: 180 },
  { colKey: 'type', title: '类型', width: 80 },
  { colKey: 'size', title: '大小', width: 90 },
  { colKey: 'create_time', title: '创建时间', width: 170 },
  { colKey: 'snap_action', title: '操作', width: 130, align: 'center' as const },
]

const snap = reactive<{
  visible: boolean
  loading: boolean
  rows: InstanceSnapshotInfo[]
  name: string
  type: string
  restoreVisible: boolean
  restoreMark: string
  restoreId: string
}>({
  visible: false,
  loading: false,
  rows: [],
  name: '',
  type: 'snap',
  restoreVisible: false,
  restoreMark: '',
  restoreId: '',
})

async function loadSnapshots(withLoading = true) {
  if (withLoading) snap.loading = true
  try {
    snap.rows = (await getInstanceSnapshots(instanceId)) || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载快照失败')
  } finally {
    snap.loading = false
  }
}

async function handleCreateSnapshot() {
  snap.loading = true
  try {
    await createInstanceSnapshot(instanceId, { type: snap.type, name: snap.name || undefined })
    MessagePlugin.success('快照创建指令已提交（面板异步执行，稍后刷新查看）')
    snap.name = ''
    setTimeout(() => loadSnapshots(false), 5000)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建快照失败')
  } finally {
    snap.loading = false
  }
}

function openRestoreSnapshot(row: InstanceSnapshotInfo) {
  snap.restoreId = row.id
  snap.restoreMark = ''
  snap.restoreVisible = true
}

async function handleRestoreSnapshot() {
  try {
    await restoreInstanceSnapshot(instanceId, { snapshot_id: snap.restoreId, confirm_mark: snap.restoreMark.trim() })
    MessagePlugin.success('恢复指令已提交')
    snap.restoreVisible = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复失败')
  }
}

async function handleDeleteSnapshot(row: InstanceSnapshotInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除快照',
    body: `确认删除「${row.name}」？删除后无法用它恢复。`,
    theme: 'danger',
    onConfirm: async () => {
      try {
        await deleteInstanceSnapshot(instanceId, row.id)
        MessagePlugin.success('快照已删除')
        loadSnapshots()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

// ===== 硬件变更 =====

const hw = reactive<{
  visible: boolean
  in_bw: number
  out_bw: number
  ip_version: number
  ip_num: number
  ip_group: string
  disk_size: number
  store: string
  loading: boolean
}>({
  visible: false,
  in_bw: 0,
  out_bw: 0,
  ip_version: 4,
  ip_num: 0,
  ip_group: '',
  disk_size: 0,
  store: '',
  loading: false,
})

async function handleSetBandwidth() {
  if (!hw.in_bw && !hw.out_bw) {
    MessagePlugin.warning('至少填一个方向的带宽')
    return
  }
  hw.loading = true
  try {
    await updateInstanceBandwidth(instanceId, { in_bw: hw.in_bw, out_bw: hw.out_bw })
    MessagePlugin.success('带宽已修改')
    hw.in_bw = 0
    hw.out_bw = 0
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '修改带宽失败')
  } finally {
    hw.loading = false
  }
}

async function handleAddIP() {
  if (!hw.ip_num) {
    MessagePlugin.warning('请输入数量')
    return
  }
  hw.loading = true
  try {
    await addInstanceIP(instanceId, { version: hw.ip_version, num: hw.ip_num, ip_group: hw.ip_group || undefined })
    MessagePlugin.success('IP 增加指令已提交')
    hw.ip_num = 0
    hw.ip_group = ''
  } catch (error) {
    MessagePlugin.error((error as Error).message || '增加 IP 失败')
  } finally {
    hw.loading = false
  }
}

async function handleAttachDisk() {
  if (!hw.disk_size) {
    MessagePlugin.warning('请输入数据盘容量')
    return
  }
  hw.loading = true
  try {
    await attachInstanceDisk(instanceId, { size_gb: hw.disk_size, store: hw.store || undefined })
    MessagePlugin.success('数据盘挂载指令已提交')
    hw.disk_size = 0
    hw.store = ''
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '挂载数据盘失败')
  } finally {
    hw.loading = false
  }
}

onMounted(() => {
  loadDetail()
  loadOperations()
  loadRelated()
})
</script>

<style lang="css">
@import './shared.css';
</style>
