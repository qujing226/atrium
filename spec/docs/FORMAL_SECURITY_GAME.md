# QLink 协议：形式化安全模型与推测执行证明 (Formal Security Model & Speculative Execution Proof)

> 本文档基于扩展的 Bellare-Rogaway (eBR) AKE 安全模型，引入了推测状态约束 (Speculative State Constraints)，通过博弈序列法 (Game-Hopping) 对 QLink 协议进行数学归约证明。

---

## 1. 系统模型与共识原语 (System Model & Consensus Primitives)

### 1.1 实体与状态 (Entities & States)
设 $\mathcal{P}$ 为协议参与者集合。对于任意会话实例 $\Pi(i, j, s)$，其内部状态集为 $\mathbb{S} = \{S_{idle}, S_{spec}, S_{ver}, S_{abort}\}$。

**协议原子性假设 (Atomicity Assumption)**：
协议逻辑保证状态转换与隔离缓冲区的操作（`Buffer.Clear` 或 `Buffer.Flush`）在本地逻辑中是原子性的。在 $S_{spec}$ 状态下，数据隔离闸门 (DIG) 机制强制阻断数据向应用层的交付。

### 1.2 共识驱动的解析预言机 (Consensus-Driven Resolve Oracle)
我们将底层区块链抽象为一个提供“真实性证明”的预言机 $\mathcal{O}_{resolve}$。该预言机针对不同共识协议提供统一的证明接口：

1.  **PoW 链 (如 Bitcoin)**: 返回 $(Doc, \pi_{spv}, k)$。$\pi_{spv}$ 为 Merkle 证明，$k$ 为当前确认深度。
2.  **BFT/PoS 链 (如 Cosmos)**: 返回 $(Doc, \Sigma_{qc})$。$\Sigma_{qc}$ 为 $>2/3$ 验证者的聚合签名 (Quorum Certificate)。
3.  **L2/zk-Rollup**: 返回 $(Doc, \pi_{zk})$。$\pi_{zk}$ 为数学上的有效性证明 (Validity Proof)。

客户端本地运行验证器 $V_{chain}(Doc, Proof)$。当且仅当 $V_{chain}$ 返回真时，状态机才允许从 $S_{spec}$ 跳转至 $S_{ver}$。

### 1.3 敌手模型 $\mathcal{A}$ (Adversary Model)
敌手 $\mathcal{A}$ 完全控制异步网络，并能调用以下预言机：
*   $\text{Send}(i, j, s, M)$: 注入、拦截或修改消息。
*   $\text{Corrupt}(i, t)$: 获取历史密钥。满足**新鲜性限制 (Freshness Constraint)**：不得获取测试会话正在使用的活跃密钥。
*   $\text{Delay}(\mathcal{O}_{resolve}, \Delta t)$: 拦截验证证明的投递，延长推测窗口。

---

## 2. 安全目标：最终应用层认证完整性 (EALA Security Game)

**定义 1 (EALA 安全性)**：若对于任意 PPT 敌手 $\mathcal{A}$，其赢得以下挑战博弈的优势 $\text{Adv}_{\mathcal{A}}^{EALA}$ 是受限且随确认深度 $k$ 衰减的，则判定协议满足 EALA 安全性。

**EALA 博弈过程 ($Game_{EALA}$)**：
1.  **Challenge(i, j, s)**: $\mathcal{A}$ 发送伪造密文序列 $C_{fake}$ 给进入 $S_{spec}$ 状态的节点 $i$。
2.  **Victory Condition**: $\mathcal{A}$ 获胜当且仅当节点 $i$ 执行了 $\text{Deliver}(P^*)$，且 $P^*$ 未经真实发送方 $j$ 授权。

---

## 3. 安全性归约证明 (Formal Proof via Game-Hopping)

**定理 1**: 假设 Ed25519 满足 EUF-CMA，Kyber768 满足 IND-CCA2，且底层共识协议具有最终确定性，则敌手 $\mathcal{A}$ 赢得 EALA 博弈的优势上界为：

$$ \text{Adv}_{\mathcal{A}}^{S-AKE} \le \epsilon_{sig} + \epsilon_{kem} + P_{reorg}(k) + \epsilon_{consensus} $$

### Game 0, 1, 2: 密码学归约
通过排除纯数学伪造签名 ($\epsilon_{sig}$) 和暴力破解密文 ($\epsilon_{kem}$) 的可能性，我们将博弈归约为对身份一致性的攻击。
$$ | \Pr[W_2] - \Pr[W_0] | \le \epsilon_{sig} + \epsilon_{kem} $$

### Game 3: 证明验证归约 (Proof-Carrying Verification)
在 $G_3$ 中，敌手 $\mathcal{A}$ 唯一的获胜路径是：提供一个能够通过 $V_{chain}$ 验证的非法证明。基于底层共识协议的属性，这种情况仅在以下两种情形发生：

1.  **共识安全性失效 (Consensus Failure)**: 敌手控制了链的共识，能够生成合法的聚合签名 $\Sigma_{qc}$ 或伪造有效性证明 $\pi_{zk}$。此概率记为 $\epsilon_{consensus}$。
2.  **状态重组 (Reorg Attack)**: 敌手利用 PoW 等协议的概率最终性，在客户端收到 $k$ 深度证明后，通过挖出更长链来回滚该状态。此概率记为 $P_{reorg}(k)$。

由于在 $S_{spec}$ 阶段数据被 DIG 机制严格隔离，只有当验证器 $V_{chain}$ 返回真时数据才会被释放。因此：
$$ \Pr[W_3 \mid \text{EALA violation}] \le P_{reorg}(k) + \epsilon_{consensus} $$

### 结论
QLink 的安全性不依赖于对查询节点的“诚实假设”，而是约化为对“共识证明”的数学校验。协议利用 DIG 机制将共识延迟 ($\Delta t$) 转化为应用层不可感知的静默等待，从而在不损失最终安全性的前提下实现了 0-RTT 的极速响应。定理得证。 $\blacksquare$
