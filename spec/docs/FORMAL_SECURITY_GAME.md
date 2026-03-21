# QLink 协议：形式化安全模型与推测执行证明 (Formal Security Model & Speculative Execution Proof)

> 本文档基于扩展的 Bellare-Rogaway (eBR) AKE 安全模型，引入了推测状态约束 (Speculative State Constraints) 和棘轮演化博弈 (Ratchet Security Game)，通过博弈序列法 (Game-Hopping) 对 QLink 协议进行数学归约证明。
> 注意：文档采用纯文本与 Unicode 符号表述数学公式以确保跨平台可读性。

---

## 1. 扩展 eBR 安全模型定义 (Extended eBR Security Model)

### 1.1 协议实例与会话标识 (Protocol Instances & Session Identifiers)
设 P 为协议参与者集合。每个参与者 i 可以并发执行多个协议实例，记为 Π(i, s)，其中 s 为本地会话索引。

**会话标识符 (Session ID, sid)**:
一个唯一的字符串，用于绑定一次特定的密钥交换执行。定义为交互报文的哈希：
$sid = 	ext{Hash}(N_{init} \parallel N_{confirm} \parallel pk_i \parallel pk_j \parallel ct)$

**匹配会话 (Matching Sessions / Partner Function)**:
若实例 Π(i, s) 与 Π(j, t) 满足以下条件，则称它们互为伙伴 (Partners)：
1. 二者计算出的 `sid` 完全一致。
2. Π(i, s) 认定的对端是 j，Π(j, t) 认定的对端是 i。
3. 二者均生成了接受状态 (Accepted State)，派生出相同的根会话密钥 `SK`。

### 1.2 敌手预言机查询 (Adversary Oracle Queries)
敌手 A 是一多项式时间 (PPT) 算法，可通过以下预言机与系统交互：

*   **Send(i, s, M)**: 向实例 Π(i, s) 发送消息 M 并获取其响应。这模拟了 A 对网络的完全控制。
*   **Corrupt(i)**: 泄露参与者 i 的长期签名私钥 (Ed25519)。
*   **Reveal(i, s)**: 泄露实例 Π(i, s) 派生出的当前会话密钥 `SK`。模拟会话级别的短暂泄露。
*   **ResolveDelay(DID, Δt)**: 专门针对身份解析预言机 $\mathcal{O}_{resolve}$ 的网络操纵。允许 A 拦截并延迟针对特定 DID 的身份真实性证明的投递，从而人为延长推测窗口。
*   **Test(i, s)**: 安全博弈的核心查询（仅限调用一次）。
    *   条件：Π(i, s) 必须是 **新鲜的 (Fresh)**（即未被 Corrupt 或 Reveal 直接攻破）。
    *   操作：挑战者 C 抛一枚公平硬币 b ∈ {0,1}。若 b=0，返回真实的 `SK`；若 b=1，返回与 `SK` 同等长度的随机均匀字符串。A 需要输出猜测 b'。

---

## 2. 安全目标定义 (Security Definitions)

### 2.1 语义安全与机密性 (Semantic Security / Key Indistinguishability)
定义协议满足语义安全，若对于任意 PPT 敌手 A，其在 Test 查询中猜测正确的优势是可忽略的：
$$ 	ext{Adv}_{\mathcal{A}}^{IND} = \left| \Pr[\mathcal{A} 	ext{ outputs } b' = b] - rac{1}{2} ight| \le \epsilon_{crypto} $$

### 2.2 最终应用层认证完整性 (EALA)
EALA 是 QLink 协议对传统 AKE 认证目标的重大范式转移。我们将安全目标从“阻止非法信道的建立 (Channel Authenticity)”转变为“阻止非法信道数据的交付 (Delivery Authenticity)”。

**定义 1 (EALA 安全性)**：若对于任意 PPT 敌手 A，其赢得以下博弈的优势是受限的：
1. A 利用 Send(i, s, C_fake) 向进入 SPECULATIVE 状态的 Π(i, s) 注入伪造密文。
2. A 获胜条件：节点 i 执行了 Deliver(P*)，且 P* 解密自 C_fake，且该消息未记录在真实 j 的发送日志中。

为了支撑 EALA 证明，我们必须形式化定义**数据隔离闸门 (DIG) 的状态转换系统 (Formal Transition System)**。定义状态演化函数 $\delta: \mathbb{S} \times Event \to \mathbb{S} \times Action$：

*   **$\delta(S_{idle}, \text{Cache\_Hit}) \to (S_{spec}, \text{Init\_Keys})$**
*   **$\delta(S_{spec}, \text{Recv\_Msg}) \to (S_{spec}, \text{Buffer.Push(Msg)})$**  *(核心：拦截交付)*
*   **$\delta(S_{spec}, \text{Proof\_Valid}) \to (S_{ver}, \text{DeliverAll(Buffer)})$** *(原子性释放)*
*   **$\delta(S_{spec}, \text{Proof\_Invalid}) \to (S_{abort}, \text{Clear(Buffer)})$** *(原子性熔断)*

---

## 3. Q-Ratchet 的形式化安全模型 (Ratchet Security Game)

Q-Ratchet 是 QLink 的状态演化机制。设内部状态为 St_i，消息密钥为 MK_i。
演化函数：$ (St_{i+1}, MK_i) \leftarrow 	ext{Ratchet}(St_i) $

为了刻画 Q-Ratchet 的混合设计，我们引入附加的预言机：
*   **StateReveal(i, s, t)**: 泄露实例 Π(i, s) 在第 t 步棘轮时的内部状态 `St_t`。

### 3.1 前向安全 (Forward Secrecy, PFS)
**定义**: 若 A 调用 `StateReveal(i, s, t)` 获取了 `St_t`，她仍无法在 `Test(i, s, t')` (其中 t' < t) 中取得不可忽略的优势。
*   **证明支撑**: 基于对称 Hash 函数（如 HMAC-SHA256）的单向性与抗原像攻击特性。

### 3.2 基于动态阈值的后向安全 (Post-Compromise Security, PCS)
纯哈希棘轮不具备 PCS。QLink 引入 **基于概率熵衰减的 Epoch-KEM 注入 (Adaptive Ratchet Rotation)**。
*   **定量风险模型 (Mathematical Risk Model)**: 设距离上次 KEM 注入已过去时间 $\Delta t_{kem}$，期间发送了 $n$ 条消息。我们定义基于时间的侧信道泄露系数为 $\lambda_t$，基于数据量的统计学分析系数为 $\lambda_n$。状态被敌手提取的累计风险函数定义为指数衰减模型：
    $$ Risk(\Delta t_{kem}, n) = 1 - \exp(-(\lambda_t \cdot \Delta t_{kem} + \lambda_n \cdot n)) $$
*   **轮换策略 (Rotation Policy)**: 定义安全上限阈值 $\Theta_{risk}$。当 $Risk(\Delta t_{kem}, n) > \Theta_{risk}$ 时，协议强制执行一次全新的 Kyber768 KEM 握手，将新的共享秘密混入当前 `St`，清零 $\Delta t_{kem}$ 与 $n$。
*   **定义 (Epoch-PCS)**: 若 A 调用 `StateReveal(i, s, t_{leak})` 获取了状态，但在 $t_{recover}$ 时刻协议执行了基于上述风险模型的 Epoch-KEM 轮换，则对于任意 $t' > t_{recover}$，A 在 `Test(i, s, t')` 中的优势再次降为可忽略。

---

## 4. 安全性归约证明概要 (Proof Sketch via Game-Hopping)

定理 1: 假设 Ed25519 满足 EUF-CMA，Kyber768 满足 IND-CCA2，且底层共识协议提供最终确定性 F(k)，则 QLink 满足 EALA 认证性及 IND-CCA2 机密性。

### Game 0 (真实博弈)
A 攻击真实协议。

### Game 1 (剥夺伪造能力)
修改挑战者：若 A 提交了未经诚实方生成的合法签名，触发 Abort。基于 EUF-CMA 假设，差值为 ε_sig。
*   此时，我们保证了匹配会话 (Matching Sessions) 的前提：只要验证通过，通信方必定是预期的 Owner。

### Game 2 (剥夺密文区分能力)
在 Test 查询中，将 Kyber 真实输出替换为理想随机数。基于 IND-CCA2 假设，差值为 ε_kem。
*   此时，证明了协议的基础 Semantic Security。

### Game 3 (证明携带与推测隔离)
处理 EALA 违规。A 唯一的路径是提供非法的有效性证明（如过期的 SPV/QC）。
由于 DIG 机制限制 `Deliver` 仅在状态跳转为 VERIFIED 后执行，A 成功的概率被严格约束于共识系统的故障率：
$$ \Pr[	ext{EALA violation}] \le P_{reorg}(k) + \epsilon_{consensus} $$

### 结论
QLink 协议在满足标准 AKE 的 IND-CCA2 与 PFS 属性的同时，通过 DIG 与参数化验证机制，将异步网络的延迟漏洞归约为了底层共识的鲁棒性问题。
