# QLink 协议：理论框架与核心能力 (Theoretical Framework & Core Capabilities)

> **学术发表的形式化安全模型 (Formal Security Model)**
> 本文档概述了 QLink 协议的理论贡献。通过引入“证明携带型解析 (Proof-Carrying Resolution)”与“数据隔离闸门 (DIG)”，协议实现了在异步不确定网络环境下的最终应用层认证完整性 (EALA)。

---

## 1. 理论贡献 A：S-AKE (推测式认证密钥交换)

### 1.1 问题定义：安全-时延悖论 (Security-Latency Paradox)
在去中心化身份 (DID) 网络中，身份验证依赖于分布式账本的共识确认。
*   **Safety (安全性)**：要求等待账本终局性确认 (Consensus Finality)，以排除公钥撤销风险。
*   **Liveness (活性)**：要求即时响应 (0-RTT)，提供无缝的用户体验。

传统协议将验证视为建立连接的前置条件。QLink 提出将身份验证解耦为“证明携带型”的异步过程。

### 1.2 核心机制：推测状态与数据隔离闸门 (Speculative State & DIG)

**定义 (推测状态 $\hat{s}$)**：协议在完成加密握手后，尚未获得共识证明的中间状态。该状态允许全双工的密文传输与 Ratchet 状态同步，但通过 **数据隔离闸门 (Data Isolation Gate, DIG)** 物理阻断明文向应用层的交付。

**协议固有规则 (Normative Rules):**
1.  **证据驱动验证 (Evidence-Driven)**：节点不依赖对端声明，必须获取来自共识层的真实性证明 (Proof of Authenticity)，如 SPV 证明、聚合签名 QC 或 ZK 证明。
2.  **强制性隔离 (Mandatory Isolation)**：在 $S_{spec}$ 状态下，所有解密后的明文必须留存在隔离缓冲区 (Isolation Buffer) 中。
3.  **原子性收敛 (Atomic Convergence)**：一旦本地验证器 (Verifier) 确认证明合法，系统必须原子性地执行：[状态升级] $\to$ [缓冲区按序推送] $\to$ [开启实时交付]。

### 1.3 形式化一致性概率模型 (Formal Consistency Model)

#### 1.3.1 推测状态的形式化定义
设 $\mathbb{S}$ 为所有已获得共识终局性确认的状态集。一个推测状态 $\hat{s} \in \hat{\mathbb{S}}$ 满足：
1.  **非终局性**: $\hat{s} \notin \mathbb{S}$。
2.  **缓存依赖性**: $\hat{s}$ 依赖本地缓存 $\mathcal{C}$，假设 $\mathcal{C} \cong \mathcal{L}$ ($\mathcal{L}$ 为当前账本真实状态)。
3.  **冲突回滚**: 若检测到 $\mathcal{C} \neq \mathcal{L}$，推测状态 $\hat{s}$ 被立即丢弃，会话转换至终止态 $S_{abort}$。

#### 1.3.2 概率一致性界限 (Probabilistic Consistency Bounds)
我们引入一致性概率函数 $P(\Delta t)$ 来刻画风险窗口：
$$ P(\Delta t) = \Pr[ \text{攻击者在 } \Delta t \text{ 时间内利用过期凭证成功欺骗应用层} \mid \text{会话处于 } S_{spec} ] $$

由于 DIG 机制的存在：
*   **定理**: 若 $t < T_{chain}$ (验证证明到达时间)，虽然加密信道可能被敌手建立，但由于明文未 `Deliver`，敌手无法诱导用户执行具有副作用的操作。
*   **结论**: 系统的应用层风险 $P(\Delta t)$ 仅取决于“共识证明被伪造”或“极短时间内的链重组”概率，趋近于密码学意义上的可忽略极小值 $\epsilon$。

---

## 2. 理论贡献 B：Q-Ratchet (轻量级后量子前向安全)

### 2.1 混合熵注入 (Hybrid Entropy-Injection)
针对 Kyber 等后量子算法密文尺寸大的特性，QLink 采用混合演化模型：
1.  **数据包级 (Packet-Level)**: 使用对称哈希链 (HKDF chain) 实现 $O(1)$ 计算开销的前向安全 (PFS)。
2.  **周期级 (Epoch-Level)**: 周期性 $N$ 注入 KEM 握手，引入新鲜量子熵，实现后向安全性 (PCS)。

### 2.2 基于概率熵衰减的自适应轮换模型 (Adaptive Ratchet Rotation via Entropy Decay)

纯哈希棘轮虽具有前向安全性，但在状态被提取（State Compromise）后无法实现后向安全自愈。为此，我们引入一种可量化的**熵衰减模型 (Entropy Decay Model)** 来驱动 Epoch-KEM 的自适应注入。

**定量风险模型 (Mathematical Risk Model)**：
假设协议状态由于内存扫描或侧信道攻击导致的泄露服从泊松分布。设当前距离上一次 KEM 注入已过去时间 $\Delta t_{kem}$，且在此期间系统处理了 $n$ 条加密消息。
我们定义**时间驱动的泄露系数 $\lambda_t$** 与 **事件驱动（侧信道）的泄露系数 $\lambda_n$**。系统状态被未知敌手提取的累积风险函数定义为：
$$ Risk(\Delta t_{kem}, n) = 1 - \exp(-(\lambda_t \cdot \Delta t_{kem} + \lambda_n \cdot n)) $$

**线性安全预算与自适应触发 (Linear Security Budget & Trigger)**：
协议设定一个安全容忍上限阈值 $\Theta_{risk}$。当 $Risk > \Theta_{risk}$ 时，必须强制执行 Epoch-KEM 轮换以注入新鲜量子熵。
为适配资源受限设备，我们对触发不等式两边取对数进行线性化推导：
$$ 1 - \exp(-(\lambda_t \cdot \Delta t_{kem} + \lambda_n \cdot n)) > \Theta_{risk} $$
$$ \lambda_t \cdot \Delta t_{kem} + \lambda_n \cdot n > -\ln(1 - \Theta_{risk}) $$
令常数安全预算 $C = -\ln(1 - \Theta_{risk})$，协议状态机只需维护一个简单的线性积分器：
$$ \text{Accumulated\_Leakage} = \lambda_t \cdot \Delta t_{kem} + \lambda_n \cdot n $$
当 $\text{Accumulated\_Leakage} > C$ 时，协议自动触发新一轮的 Kyber768 握手，重置 $\Delta t_{kem}$ 与 $n$。

此模型在不改变底层密码学算法的前提下，通过系统级的调度策略，实现了后向量子安全 (PCS) 与带宽开销的数学最优解。

---

## 3. 核心能力总结

| 能力 | 工程视角 (实现) | 科学视角 (协议理论) |
| :--- | :--- | :--- |
| **0-RTT 启动** | 基于本地缓存的乐观执行 | **S-AKE**: 针对高延迟信任锚点的推测执行模型。 |
| **隔离防御** | 应用层数据隔离与拦截 | **Data Isolation Gate**: 推测窗口内的交付阻断策略。 |
| **确权升级** | 轻客户端证明 (SPV/QC) 校验 | **Proof-Carrying Resolution**: 基于共识证据的状态机确权转换。 |
| **PFS (前向安全)**| 基于对称哈希的密钥滚动 | **Q-Ratchet**: 面向受限环境的、混合熵注入的密钥演化函数。 |
