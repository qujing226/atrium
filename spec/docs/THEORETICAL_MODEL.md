# QLink 协议：理论框架与核心能力

> **学术发表的形式化安全模型 (Formal Security Model)**
>
> 本文档概述了 QLink 协议的理论贡献。它通过定义适合顶级学术研究 (SCI Q1) 的规范性规则和数学模型，将“推测执行”引入 AKE 协议的生命周期管理中。

---

## 1. 理论贡献 A：S-AKE (推测式认证密钥交换)

### 1.1 问题定义：安全-时延悖论 (Security-Latency Paradox)

在去中心化身份 (DID) 网络中，信任的锚点（区块链）通常具有显著的共识延迟 $T_{chain}$。
*   **Safety (安全性)**：要求等待账本确认，确保公钥未被撤销。
*   **Liveness (活性)**：要求即时响应（0-RTT），提供流畅的用户体验。

传统的 AKE 协议（如 TLS 1.3）将身份验证作为建立会话的“前置条件”。QLink 提出将身份验证解耦为“后置异步过程”。

### 1.2 核心机制：推测状态与数据闸门 (Speculative State & Data Gate)

**定义 (推测状态 $\hat{s}$)**：协议在建立加密信道后，尚未完成最终身份确权的中间状态。该状态允许数据的加解密与传输，但通过 **“数据闸门” (Data Gate)** 限制敏感信息的交付。

**协议固有规则 (Normative Rules):**
1.  **Autonomous Verification (自主验证)**：节点独立向区块链查询 $FreshDoc$，而不信任对端声明的版本。
2.  **Isolated Buffer (隔离缓冲区)**：在推测期间，$S_{spec}$ 状态下的解密明文必须被隔离在缓冲区中，禁止流向应用层。
3.  **Atomic State Convergence (原子收敛)**：一旦后台验证通过，必须原子性地完成：[升级状态] -> [开放闸门] -> [按序推送缓冲区消息]。

### 1.3 协议推测状态的形式化安全模型 (Formal Security Model)

#### 1.3.1 推测状态的形式化定义 (Formal Definition)

设 $\mathbb{S}$ 为所有 **账本确认状态 (Ledger-Confirmed States)** 的集合。一个推测状态 $\hat{s} \in \hat{\mathbb{S}}$ 具有以下三个关键特征：

1.  **非终局性 (Non-Finality)**: $\hat{s} \notin \mathbb{S}$。
2.  **缓存依赖性 (Cache Dependency)**: $\hat{s}$ 的生成依赖于本地 DID 缓存 $\mathcal{C}$，并基于一个乐观假设：$\mathcal{C} \cong \mathcal{L}$ (账本状态)。
3.  **原子回滚语义 (Atomic Rollback Semantics)**: 若检测到 $\mathcal{C} \neq \mathcal{L}$，推测状态 $\hat{s}$ 被立即丢弃，会话回滚至终止态 $s \in \mathbb{S}$。

因此，推测状态集 $\hat{\mathbb{S}}$ 是有效状态集 $\mathbb{S}$ 的超集，满足 $\hat{\mathbb{S}} \supset \mathbb{S}$。

#### 1.3.2 一致性概率模型 (Consistency Probability Model)

我们引入 **一致性概率函数** $P(\Delta t)$。设 $\Delta t$ 为推测窗口的持续时间。

**定义：**
$$P(\Delta t) = \Pr[\text{攻击者在 } \Delta t \text{ 时间窗口内利用过期凭证成功欺骗应用层} \mid \hat{s} \text{ 处于活跃状态}]$$

由于协议引入了 **数据闸门机制**，明文在 $\Delta t$ 窗口内仅存在于隔离缓冲区中。
*   **结论 1**: 若 $t < T_{chain}$，虽然 $\hat{s}$ 是推测的，但由于数据未交付应用层，攻击者无法通过过期凭证诱导用户执行错误决策。
*   **结论 2**: $P(\Delta t) \approx 0$。S-AKE 机制本质上是利用 $P(\Delta t)$ 的极低风险，交换了 $O(T_{chain})$ 的性能增益。

---

## 2. 理论贡献 B：Q-Ratchet (轻量级后量子前向安全)

### 2.1 混合熵注入 (Hybrid Entropy-Injection)

**定义**：Q-Ratchet 结合了对称哈希的高效性与 KEM 的量子安全性。

1.  **Packet-Level (数据包级)**: 使用轻量级 HKDF 进行每消息的密钥滚动，实现前向安全。
2.  **Epoch-Level (时代级)**: 周期性引入 KEM 重新握手（注入新的熵），实现后向安全性 (Post-Compromise Security)。

### 2.2 数学模型：熵衰减与自愈 (Entropy Decay & Self-Healing)

我们对系统的“前向安全强度” $S(t)$ 进行建模。
*   **纯哈希链**：$S(t)$ 随时间线性衰减，一旦状态泄露，后续通信无自愈性。
*   **Q-Ratchet**：通过注入频率 $N$，维持 $S(t) > Threshold$ 的锯齿形自愈模型。

---

## 3. 核心能力总结

| 能力 | 工程视角 (实现) | 科学视角 (协议理论) |
| :--- | :--- | :--- |
| **0-RTT 启动** | "我查了一下本地缓存。" | **S-AKE**: 针对高延迟信任锚点的推测执行模型。 |
| **隔离防御** | "解密但不交给 App。" | **Data Gate**: 推测窗口内的应用层明文隔离策略。 |
| **快速失效** | "收到错误包立即断开。" | **Proactive Rollback**: 故障的主动同步与最终一致性保证。 |
| **PFS (前向安全)**| "我每次都 Hash 密钥。" | **Q-Ratchet**: 面向受限环境的、混合熵注入的密钥演化函数。 |
