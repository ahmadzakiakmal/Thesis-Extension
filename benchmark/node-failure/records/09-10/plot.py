import pandas as pd
import matplotlib.pyplot as plt
import glob
import os
import re

# Directory containing the CSV files
records_dir = '.'

# Find all CSV files
csv_files = sorted(glob.glob(f'{records_dir}/node_failure_*.csv'))

if not csv_files:
    print(f"❌ No CSV files found in {records_dir}/")
    exit(1)

print(f"📊 Found {len(csv_files)} configuration files")
print("=" * 70)

# Store results for each configuration
configs = []
baseline_means = []
baseline_stds = []
failed_means = []
failed_stds = []

for csv_file in csv_files:
    # Extract L1 and L2 node counts from filename
    match = re.search(r'l1-(\d+)_l2-(\d+)', csv_file)
    if not match:
        print(f"⚠️  Skipping {csv_file} - couldn't parse config")
        continue
    
    l1_nodes = int(match.group(1))
    l2_nodes = int(match.group(2))
    config_name = f"{l1_nodes}-{l2_nodes}"
    
    print(f"Processing: {config_name} ({os.path.basename(csv_file)})")
    
    # Load and process data
    df = pd.read_csv(csv_file)
    df_filtered = df[df['Success'].astype(str).str.lower() == 'true'].copy()
    df_filtered['Latency_ms'] = pd.to_numeric(df_filtered['Latency_ms'], errors='coerce')
    
    # Separate phases
    baseline = df_filtered[df_filtered['Phase'] == 'baseline']
    failed = df_filtered[df_filtered['Phase'] == 'node-failed']
    
    # Get Commit Session stats
    baseline_commit = baseline[baseline['Step'] == 'Commit Session']['Latency_ms']
    failed_commit = failed[failed['Step'] == 'Commit Session']['Latency_ms']
    
    if len(baseline_commit) > 0 and len(failed_commit) > 0:
        configs.append(config_name)
        baseline_means.append(baseline_commit.mean())
        baseline_stds.append(baseline_commit.std())
        failed_means.append(failed_commit.mean())
        failed_stds.append(failed_commit.std())
        
        print(f"  ✓ Baseline: {baseline_commit.mean():.0f}ms ± {baseline_commit.std():.0f}ms")
        print(f"  ✓ Failed:   {failed_commit.mean():.0f}ms ± {failed_commit.std():.0f}ms")
    else:
        print(f"  ✗ Insufficient data")
    print()

print("=" * 70)
print(f"✅ Successfully processed {len(configs)} configurations")
print()

# Create comparison plot
fig, ax = plt.subplots(figsize=(14, 7))

x = range(len(configs))
width = 0.35

# Create bars
bars1 = ax.bar([i - width/2 for i in x], baseline_means, width,
               label='Baseline (All nodes healthy)', 
               color='#2ecc71', alpha=0.8, edgecolor='black', linewidth=1.5)
bars2 = ax.bar([i + width/2 for i in x], failed_means, width,
               label='Node Failed (1 node down)', 
               color='#e74c3c', alpha=0.8, edgecolor='black', linewidth=1.5)

# Add error bars
ax.errorbar([i - width/2 for i in x], baseline_means, yerr=baseline_stds,
            fmt='none', ecolor='#1a7a3d', capsize=5, capthick=2, linewidth=1.5)
ax.errorbar([i + width/2 for i in x], failed_means, yerr=failed_stds,
            fmt='none', ecolor='#a02020', capsize=5, capthick=2, linewidth=1.5)

# Add value labels on bars
for i, (b_mean, f_mean) in enumerate(zip(baseline_means, failed_means)):
    # Baseline label
    ax.text(i - width/2, b_mean + baseline_stds[i] + 50, f'{b_mean:.0f}',
            ha='center', va='bottom', fontsize=9, fontweight='bold')
    # Failed label
    ax.text(i + width/2, f_mean + failed_stds[i] + 50, f'{f_mean:.0f}',
            ha='center', va='bottom', fontsize=9, fontweight='bold')

# Styling
ax.set_xlabel('Configuration (L1-L2 Nodes)', fontsize=13, fontweight='bold')
ax.set_ylabel('Commit Session Latency (ms)', fontsize=13, fontweight='bold')
ax.set_title('Byzantine Fault Tolerance: Node Failure Impact Across Configurations', 
             fontsize=15, fontweight='bold')
ax.set_xticks(x)
ax.set_xticklabels(configs, fontsize=11)
ax.legend(loc='upper left', fontsize=12, framealpha=0.95)
ax.grid(axis='y', alpha=0.3, linestyle='--', linewidth=1)
ax.set_ylim(bottom=0)

plt.tight_layout()
plt.savefig(f'{records_dir}/comparison_all_configs.png', dpi=300, bbox_inches='tight')
print(f"📈 Comparison plot saved: {records_dir}/comparison_all_configs.png")

# Print summary table
print("\n" + "=" * 70)
print("SUMMARY TABLE")
print("=" * 70)
print(f"{'Config':<10} {'Baseline (ms)':<15} {'Failed (ms)':<15} {'Increase':<12}")
print("-" * 70)
for i, config in enumerate(configs):
    increase = ((failed_means[i] - baseline_means[i]) / baseline_means[i]) * 100
    print(f"{config:<10} {baseline_means[i]:>8.0f} ± {baseline_stds[i]:<4.0f} "
          f"{failed_means[i]:>8.0f} ± {failed_stds[i]:<4.0f} "
          f"+{increase:>6.1f}%")
print("=" * 70)