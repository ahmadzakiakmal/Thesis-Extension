import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import glob
import re

# Find all CSV files in current directory
csv_files = glob.glob('*.csv')
if not csv_files:
    print("❌ No CSV files found in current directory!")
    exit(1)

print(f"📊 Found {len(csv_files)} configuration files")
print()

# Dictionary to store results: {l1_nodes: {phase: [latencies]}}
configs = {}

# Process each CSV file
for csv_file in sorted(csv_files):
    # Extract L1 node count from filename
    match = re.search(r'l1-(\d+)', csv_file)
    if not match:
        print(f"⚠️  Skipping {csv_file} - couldn't extract L1 node count")
        continue
    
    l1_nodes = int(match.group(1))
    print(f"Processing: {csv_file} (L1={l1_nodes} nodes)")
    
    # Load data
    df = pd.read_csv(csv_file)
    
    # Filter only successful Commit Session steps
    df_commit = df[
        (df['Success'].astype(str).str.lower() == 'true') & 
        (df['Step'] == 'Commit Session')
    ].copy()
    
    if len(df_commit) == 0:
        print(f"  ⚠️  No successful Commit Session entries found!")
        continue
    
    # Convert to numeric
    df_commit['Latency_ms'] = pd.to_numeric(df_commit['Latency_ms'], errors='coerce')
    
    # Group by phase
    configs[l1_nodes] = {}
    for phase in ['all-healthy', '1-byzantine', '2-byzantine']:
        phase_data = df_commit[df_commit['Phase'] == phase]['Latency_ms']
        if len(phase_data) > 0:
            configs[l1_nodes][phase] = phase_data.values
            print(f"  ✓ {phase}: {len(phase_data)} samples, avg={phase_data.mean():.1f}ms")
        else:
            configs[l1_nodes][phase] = np.array([])
            print(f"  ✗ {phase}: No data (consensus failed)")
    print()

if not configs:
    print("❌ No valid configuration data found!")
    exit(1)

# Prepare data for plotting
sorted_l1_nodes = sorted(configs.keys())
phases = ['all-healthy', '1-byzantine', '2-byzantine']
phase_labels = ['All Healthy (f=0)', '1 Byzantine (f=1)', '2 Byzantine (f=2)']
colors = ['#2ecc71', '#f39c12', '#e74c3c']

# Create figure
fig, ax = plt.subplots(figsize=(14, 8))

# Plot settings
x = np.arange(len(sorted_l1_nodes))
width = 0.25

# Plot bars for each phase
for i, (phase, label, color) in enumerate(zip(phases, phase_labels, colors)):
    means = []
    stds = []
    
    for l1_nodes in sorted_l1_nodes:
        data = configs[l1_nodes].get(phase, np.array([]))
        if len(data) > 0:
            means.append(np.mean(data))
            stds.append(np.std(data))
        else:
            means.append(0)
            stds.append(0)
    
    # Plot bars with error bars
    bars = ax.bar(x + i*width - width, means, width, 
                   label=label, color=color, alpha=0.8, 
                   edgecolor='black', linewidth=1.2)
    
    # Add error bars
    ax.errorbar(x + i*width - width, means, yerr=stds, 
                fmt='none', ecolor='black', capsize=5, 
                capthick=1.5, alpha=0.6)
    
    # Add value labels on top of bars
    for j, (mean, std) in enumerate(zip(means, stds)):
        if mean > 0:  # Only show label if there's data
            label_text = f'{mean:.0f}'
            ax.text(j + i*width - width, mean + std + max(means)*0.02, 
                   label_text, ha='center', va='bottom', 
                   fontsize=9, fontweight='bold')

# Customize plot
ax.set_xlabel('L1 Node Configuration', fontsize=13, fontweight='bold')
ax.set_ylabel('L1 Consensus Latency (ms)', fontsize=13, fontweight='bold')
# ax.set_title('L1 Consensus Latency - Byzantine Fault Impact Across Node Configurations', 
#              fontsize=15, fontweight='bold', pad=20)

# Set x-axis labels with fault tolerance info
x_labels = []
for l1_nodes in sorted_l1_nodes:
    f = (l1_nodes - 1) // 3
    x_labels.append(f'{l1_nodes} nodes\n(f={f})')
ax.set_xticks(x)
ax.set_xticklabels(x_labels)

ax.legend(fontsize=11, loc='upper left')
ax.grid(axis='y', alpha=0.3, linestyle='--')

# Set y-axis limits to start at 0
ax.set_ylim(bottom=0)

# Add a horizontal line at y=0 for better readability
ax.axhline(y=0, color='black', linewidth=0.8)

plt.tight_layout()
output_file = 'l1_consensus_comparison_all_configs.png'
plt.savefig(output_file, dpi=300, bbox_inches='tight')
print(f"✅ Chart saved: {output_file}")
print()

# Print summary statistics
print("=" * 80)
print("SUMMARY STATISTICS")
print("=" * 80)
print()

for l1_nodes in sorted_l1_nodes:
    f = (l1_nodes - 1) // 3
    print(f"L1 Nodes: {l1_nodes} (f={f})")
    print("-" * 80)
    
    for phase, label in zip(phases, phase_labels):
        data = configs[l1_nodes].get(phase, np.array([]))
        if len(data) > 0:
            print(f"  {label:25s}: {np.mean(data):7.1f} ms  (±{np.std(data):6.1f} ms)  [{len(data)} samples]")
        else:
            print(f"  {label:25s}: FAILED - No consensus reached")
    print()

print("=" * 80)
print("FAULT TOLERANCE VERIFICATION")
print("=" * 80)
for l1_nodes in sorted_l1_nodes:
    f = (l1_nodes - 1) // 3
    byz1_ok = len(configs[l1_nodes].get('1-byzantine', [])) > 0
    byz2_ok = len(configs[l1_nodes].get('2-byzantine', [])) > 0
    
    print(f"L1={l1_nodes} nodes (f={f}): ", end="")
    if byz2_ok:
        print(f"✅ Can tolerate 2 Byzantine nodes")
    elif byz1_ok:
        print(f"⚠️  Can tolerate 1 Byzantine node only")
    else:
        print(f"❌ Cannot tolerate any Byzantine nodes")
print()