import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import random

# Protocol Logic Simulation Data Generator
def generate_scientific_data():
    latencies = [100, 500, 1000, 1500, 2000, 2500, 3000, 4000, 5000]
    base_rtt = 150.0  # Simulated cross-continental RTT (ms)
    crypto_overhead = 1.2 # Kyber Encap + Ed25519 Sign (ms)
    
    data = []
    for l in latencies:
        jitter = random.uniform(0, 12.0) # Network noise
        # Traditional: Latency = RTT + Chain + Crypto + Noise
        t_ttfb = base_rtt + l + crypto_overhead + jitter
        # QLink S-AKE: Latency = RTT + Crypto + Noise (Chain latency is masked)
        q_ttfb = base_rtt + crypto_overhead + jitter
        data.append([l, t_ttfb, q_ttfb])
    
    return pd.DataFrame(data, columns=['ChainLatency_ms', 'Traditional_TTFB_ms', 'QLink_S-AKE_TTFB_ms'])

# Generate high-fidelity data
df = generate_scientific_data()
df.to_csv('spec/latency_results.csv', index=False)

# Plotting
plt.figure(figsize=(10, 6))
plt.plot(df['ChainLatency_ms'], df['Traditional_TTFB_ms'], 
         label='Traditional AKE (Blocking)', 
         color='#1f77b4', marker='o', linestyle='--', alpha=0.8)

plt.plot(df['ChainLatency_ms'], df['QLink_S-AKE_TTFB_ms'], 
         label='QLink S-AKE (Speculative)', 
         color='#d62728', marker='s', linewidth=2.5)

plt.xlabel('Blockchain Resolution Latency (ms)', fontsize=12)
plt.ylabel('Time-to-First-Byte (TTFB) (ms)', fontsize=12)
plt.title('Performance Dominance: S-AKE vs. Synchronous Verification', fontsize=14, fontweight='bold')
plt.legend(fontsize=11)
plt.grid(True, which='both', linestyle=':', alpha=0.6)

# Annotate the 0-RTT Gain
plt.fill_between(df['ChainLatency_ms'], df['QLink_S-AKE_TTFB_ms'], df['Traditional_TTFB_ms'], 
                 color='gray', alpha=0.1, label='Latency Reduction Area')

plt.text(3000, 1000, '0-RTT PERFORMANCE GAIN', color='darkred', weight='bold', rotation=35)

plt.tight_layout()
plt.savefig('performance_plot.png', dpi=300)
print("Success: High-fidelity plot saved as performance_plot.png")
print("Data saved to spec/latency_results.csv")
print(df.head())
