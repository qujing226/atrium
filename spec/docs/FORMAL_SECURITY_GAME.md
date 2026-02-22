# QLink 协议形式化安全博弈模型 (Formal Security Game)

> **目标**: 证明 QLink 协议在推测执行窗口 $\Delta t$ 内，面对具备网络控制权和部分旧密钥持有权的敌手时，能够保持应用层数据的机密性与完整性。

---

## 1. 敌手模型 (Adversary Model - Eve)

我们定义一个能力强大的敌手 $\mathcal{A}$ (Eve)，她具备以下能力：

1.  **网络完全控制 (Full Network Control)**: 
    *   $\mathcal{A}$ 可以拦截、修改、丢弃、重放所有的 TCP 数据包。
    *   $\mathcal{A}$ 熟悉 QLink 的 协议编码格式。
2.  **旧密钥持有权 (Past Key Compromise)**: 
    *   假设 $\mathcal{A}$ 通过某种手段获取了通信双方在 $T_{past}$ 时刻的 Ed25519 私钥和 Kyber 私钥。
    *   $\mathcal{A}$ 可以使用这些旧私钥伪造签名和解密基于旧公钥封装的密文。
3.  **基础设施阻塞 (Infrastructure Delay)**: 
    *   $\mathcal{A}$ 控制了大部分网络节点，能够识别并延迟（甚至在有限时间内拦截）客户端向区块链发起的 `Resolve` 请求或返回的响应。
    *   这意味着 $\mathcal{A}$ 可以通过延迟查链来**人工延长**协议的推测窗口 $\Delta t$。

---

## 2. 安全性命题 (Security Claims)

### 2.1 后量子机密性 (Post-Quantum Confidentiality in $\Delta t$)
*   **命题**: 即使 $\mathcal{A}$ 持有 A 的旧签名私钥 $sk_{A,old}$ 并记录了全量历史流量，仍无法恢复推测窗口内的明文消息。
*   **依据**: 
    1.  **KEM 强绑定**: 共享种子的安全性由 B 的 Kyber768 公钥保障。只要 B 的当前加密私钥 $sk_{B,now}$ 未泄露，$\mathcal{A}$ 无法从 `KEMInit` 中提取种子。
    2.  **前向安全 (PFS)**: 即使未来的 KEM 握手密钥泄露，Q-Ratchet 保证了历史消息的解密难度等价于破解 SHA256 原像。
    3.  **量子抗性**: 算法基于 ML-KEM，抗 Shor 算法攻击。

### 2.2 推测期内的认证完整性 (Integrity & Data Gate)
*   **命题**: $\mathcal{A}$ 无法利用过期缓存公钥，在验证窗口 $\Delta t$ 结束前，将任何伪造数据注入 Bob 的应用层。
*   **依据**: 
    1.  **数据闸门 (Data Gate)**: 只要会话处于 `STATE_SPECULATIVE`，解密后的明文必须被隔离在 `PendingMsgs` 中。
    2.  **强制查链 (Mandatory Resolve)**: B 不依赖 A 声明的版本，自主启动区块链验证。
    3.  **结果**: 任何身份伪造必将在 $T_{chain}$ 后被检测，触发 $S_{abort}$ 并销毁隔离区数据。

### 2.3 最终一致性与可用性边界 (Eventual Consistency)
*   **命题**: $\mathcal{A}$ 通过阻塞网络无法无限期延长推测窗口。
*   **依据**: 
    1.  **水位线强制熔断**: 定义隔离区水位线 $B_{max}$ 和超时时间 $\tau$。
    2.  **确定性终态**: 一旦满足触发条件，会话将强制收敛至 $S_{abort}$。
    3.  **信任锚假设**: 假设区块链账本 $\mathcal{L}$ 是不可篡改且具有最终确定性的 (Eventually Final)。

---

## 3. 安全博弈过程 (The Security Game)

### 3.1 实验设置 (Setup)
1. 挑战者 $\mathcal{C}$ 初始化 Alice (A) 和 Bob (B) 的当前合法密钥对 $(pk_{now}, sk_{now})$。
2. $\mathcal{C}$ 将 A 和 B 的旧密钥 $(pk_{old}, sk_{old})$ 交给敌手 $\mathcal{A}$。
3. B 的本地缓存中存储的是 A 的旧公钥 $pk_{old}$。

### 3.2 挑战阶段 (Challenge Phase)
1.  **伪造尝试**: $\mathcal{A}$ 伪装成 A，使用泄露的 $sk_{A,old}$ 签署一个 `KEMInit` 发给 B。
2.  **推测执行**: B 收到后，查本地缓存（存储 $pk_{A,old}$）验证通过，进入 `STATE_SPECULATIVE`。
3.  **数据隔离**: B 解密 `KEMInit` 获得种子，并开始接收 $\mathcal{A}$ 发送的伪造加密消息序列 $M'_1, M'_2...$。
    *   **关键动作**: B 将解密后的明文 $P'_1, P'_2...$ 存入**线性隔离缓冲区 (Linear Isolation Buffer)**，暂不交付应用层。
4.  **异步验证**: B 同时启动后台查链任务 `Verify(A)`。$\mathcal{A}$ 尝试通过网络阻塞延迟该请求。

### 3.3 胜负判定 (Victory Condition)

我们将协议的安全性定义为 **“应用层认证完整性 (Application-Layer Authenticity)”**。

*   **敌手获胜 (Adversary Wins)**: 
    *   如果 B 的应用层回调 `OnMessage` 接收并处理了任何一条由 $\mathcal{A}$ 伪造的消息 $P'_i$。
    *   这意味着 $\mathcal{A}$ 成功绕过了异步验证机制或在该机制生效前完成了注入。

*   **协议获胜 (Protocol Wins)**: 
    *   即使 $\mathcal{A}$ 成功建立了加密通道并发送了消息，只要 B 在 $T_{chain}$ 时间内通过区块链发现公钥不匹配，并执行了 **原子回滚 (Atomic Rollback)**。
    *   回滚操作清空了隔离缓冲区中的 $P'_i$，使得 B 的应用层从未感知到这次攻击。
    *   **结论**: 只要应用层未受污染，协议即视为在认证性上取得了胜利。

---

## 4. 形式化证明草稿 (Formal Proof Sketch via Ideal Functionality)

为了在 UC (Universally Composable) 类框架下论证安全性，我们构造一个理想功能 $\mathcal{F}_{S-AKE}$，并证明真实协议 $\Pi_{QLink}$ 在允许时间延迟 $\Delta t$ 的条件下模拟了该功能。

### 4.1 理想功能 $\mathcal{F}_{S-AKE}$ (The Ideal World)
$\mathcal{F}_{S-AKE}$ 是一个连接 Alice、Bob 和 理想账本 $\mathcal{L}$ 的可信中介。

1.  **初始化**: $\mathcal{F}$ 记录双方的公钥 $pk_A, pk_B$。
2.  **发送指令**: 当 Alice 输入 `Send(M, Bob)` 时：
    *   $\mathcal{F}$ 立即查询 $\mathcal{L}$ 获取 Bob 的当前状态 $State_B$。
    *   若 $State_B$ 为 `Active` 且 $pk_B$ 匹配，$\mathcal{F}$ 将 $(Alice, M)$ 递交给 Bob。
    *   若 $State_B$ 为 `Revoked`，$\mathcal{F}$ 向 Alice 和 Bob 输出 `Abort`。
3.  **特性**: 在理想世界中，**不存在** 任何未经验证的消息交付，也不存在回滚。消息要么原子性地成功，要么失败。

### 4.2 真实协议 $\Pi_{QLink}$ (The Real World)
$\Pi_{QLink}$ 是我们在不可信网络和延迟账本上运行的协议。

1.  **发送**: Alice 基于缓存 $pk'_B$ 加密消息 $C$ 并发送。
2.  **接收与隔离**: Bob 收到 $C$，解密得 $M$，存入隔离区 `Buffer`。此时 Bob 的应用层**未收到**任何输出。
3.  **异步验证**: Bob 启动后台查询，经过延迟 $\delta \in [0, \Delta t]$ 后获得结果。
4.  **最终交付**:
    *   若验证通过，Bob 输出 `Buffer` 中的 $M$。
    *   若验证失败，Bob 销毁 `Buffer`，输出 `Abort`。

### 4.3 松弛不可区分性命题 (Relaxed Indistinguishability Theorem)

**定义**: 我们称协议 $\Pi$ 是 **$\Delta t$-延迟安全** 的，如果对于任意多项式时间的区分者 $\mathcal{Z}$ (Environment/Distinguisher)，存在一个模拟器 $\mathcal{S}$，使得：

$$ | \Pr[\mathcal{Z}(\Pi) = 1] - \Pr[\mathcal{Z}(\mathcal{F}) = 1] | \le \epsilon_{crypto} + P(\Delta t) $$

其中：
*   **$\epsilon_{crypto}$**: 攻破 Kyber 或 Hash 函数的概率（可忽略）。
*   **$P(\Delta t)$**: 在推测窗口 $\Delta t$ 内，链上状态发生针对性突变（Targeted Mutation）且网络被精确操控导致 $\mathcal{Z}$ 能区分“延迟交付”与“即时交付”差异的概率。

**证明思路 (Sketch)**:
1.  **观察等价性**: 在 $t < \Delta t$ 期间，$\mathcal{F}$ 什么都不输出（因为它瞬间完成检查并发现需要等待），$\Pi$ 也不输出（因为数据在 Buffer 中）。对于应用层，两者表现一致（Silence）。
2.  **终态一致性**: 在 $t \ge \Delta t$ 时，$\Pi$ 完成了查链。
    *   如果链上正常：$\Pi$ 释放 $M$，$\mathcal{F}$ 输出 $M$。一致。
    *   如果链上撤销：$\Pi$ 销毁 $M$ 输出 `Abort`，$\mathcal{F}$ 早就输出了 `Abort`。一致。
3.  **结论**: 只要**数据闸门**严格执行，$\mathcal{Z}$ 无法通过观察应用层的输出来区分自己是在真实世界还是理想世界，除非它能攻破底层的密码学原语。
