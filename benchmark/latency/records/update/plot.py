#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os

# Baseline data from the old system (with consensus between L2 nodes)
baseline_data = {
    '4-1': 58.3,
    '4-2': 203.1,
    '4-3': 221.1,
    '4-4': 247.0
}

# Read the new sharded L2 data
records_dir = './'
csv_files = {
    '4-1': 'latency_2025-10-06_12-58-00_n100_l1-4_l2-1.csv',
    '4-2': 'latency_2025-10-06_12-55-34_n100_l1-4_l2-2.csv',
    '4-3': 'latency_2025-10-06_13-00-10_n100_l1-4_l2-3.csv',
    '4-4': 'latency_2025-10-06_13-02-55_n100_l1-4_l2-4.csv'
}

# Extract average L2 latency from each CSV
# L2 = all steps EXCEPT "Commit Session" (which is L1) and "Complete Workflow" (which is total)
new_data = {}
for config, filename in csv_files.items():
    filepath = os.path.join(records_dir, filename)
    df = pd.read_csv(filepath)
    
    # Filter out "Commit Session" and "Complete Workflow" to get only L2 steps
    l2_steps = df[(df['Step'] != 'Commit Session') & (df['Step'] != 'Complete Workflow')]
    avg_latency = l2_steps['Latency_ms'].mean()
    new_data[config] = avg_latency
    print(f"{config}: Avg L2 Latency = {avg_latency:.1f} ms (excluding Commit & Complete)")

# Prepare data for plotting
configs = ['4-1', '4-2', '4-3', '4-4']
baseline_values = [baseline_data[c] for c in configs]
new_values = [new_data[c] for c in configs]

# Create the comparison figure
fig, ax = plt.subplots(figsize=(8, 6))

# Set up bar positions
x = np.arange(len(configs))
bar_width = 0.35

# Color scheme from repository
baseline_color = '#fc9272'  # Red/salmon for baseline (old with consensus)
new_color = '#c6dbef'  # Blue for new (sharded, independent L2)

# Create bars
bars1 = ax.bar(x - bar_width/2, new_values, bar_width, 
               label='L2 Sharded (New)', color=new_color, 
               edgecolor='black', linewidth=0.8)
bars2 = ax.bar(x + bar_width/2, baseline_values, bar_width, 
               label='L2 Consensus (Baseline)', color=baseline_color, 
               edgecolor='black', linewidth=0.8)

# Add value labels on bars
for bar in bars1:
    height = bar.get_height()
    ax.text(bar.get_x() + bar.get_width()/2., height,
            f'{height:.1f}', ha='center', va='bottom', 
            fontsize=11, fontweight='bold')

for bar in bars2:
    height = bar.get_height()
    ax.text(bar.get_x() + bar.get_width()/2., height,
            f'{height:.1f}', ha='center', va='bottom', 
            fontsize=11, fontweight='bold')

# Customize plot
ax.set_xlabel('Node Configuration (L1-L2)', fontsize=15, labelpad=10)
ax.set_ylabel('Latency (ms)', fontsize=15, labelpad=10)
ax.set_title('Layer 2 Average Latency:\nSharded vs Baseline Architecture', 
             fontsize=16, pad=15)
ax.set_xticks(x)
ax.set_xticklabels(configs, fontsize=13)
ax.tick_params(axis='y', labelsize=12)

# Add legend
ax.legend(fontsize=12, loc='upper left', frameon=True, 
          framealpha=0.9, edgecolor='lightgray')

# Add grid
ax.yaxis.grid(True, linestyle='--', alpha=0.5, zorder=0)

# Set y-axis limits
ax.set_ylim(0, max(max(baseline_values), max(new_values)) * 1.15)

# Add note about the difference
improvement = {}
for config in configs:
    old = baseline_data[config]
    new = new_data[config]
    improvement[config] = ((old - new) / old) * 100

print("\n" + "="*60)
print("LATENCY IMPROVEMENT (Sharded vs Baseline)")
print("="*60)
for config in configs:
    print(f"{config}: {improvement[config]:+.1f}% "
          f"({'faster' if improvement[config] > 0 else 'slower'})")
print("="*60)

# Save figure
plt.tight_layout()
plt.savefig('l2_latency_comparison.png', dpi=300, bbox_inches='tight')
plt.savefig('l2_latency_comparison.pdf', bbox_inches='tight')
print("\n✓ Figures saved:")
print("  - l2_latency_comparison.png")
print("  - l2_latency_comparison.pdf")

plt.show()