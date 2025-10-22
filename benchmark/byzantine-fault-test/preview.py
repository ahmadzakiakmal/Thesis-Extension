import pandas as pd
import matplotlib.pyplot as plt
import glob
import os

# Find the latest CSV file
csv_files = glob.glob('records/byzantine_fault_*.csv')
if not csv_files:
    print("❌ No CSV files found in records/")
    exit(1)

latest_csv = max(csv_files, key=os.path.getctime)
print(f"📊 Analyzing: {latest_csv}")
print()

# Load data
df = pd.read_csv(latest_csv)

# Debug: Print initial info
print("=" * 70)
print("DEBUG INFO")
print("=" * 70)
print(f"Total rows: {len(df)}")
print(f"Columns: {df.columns.tolist()}")
print(f"Success values: {df['Success'].unique()}")
print(f"Phases: {df['Phase'].unique()}")
print()

# Filter only successful requests
df_filtered = df[df['Success'].astype(str).str.lower() == 'true'].copy()
print(f"Rows after filtering for success: {len(df_filtered)}")
print()

if len(df_filtered) == 0:
    print("❌ No successful requests found in the CSV!")
    print("First few rows of raw data:")
    print(df.head(10))
    exit(1)

# Convert Latency_ms to numeric
df_filtered['Latency_ms'] = pd.to_numeric(df_filtered['Latency_ms'], errors='coerce')

# Separate phases
all_healthy = df_filtered[df_filtered['Phase'] == 'all-healthy'].copy()
byz1 = df_filtered[df_filtered['Phase'] == '1-byzantine'].copy()
byz2 = df_filtered[df_filtered['Phase'] == '2-byzantine'].copy()

print(f"All Healthy rows: {len(all_healthy)}")
print(f"1 Byzantine rows: {len(byz1)}")
print(f"2 Byzantine rows: {len(byz2)}")
print()

if len(all_healthy) == 0 or len(byz1) == 0 or len(byz2) == 0:
    print("❌ Missing data for one or more phases!")
    exit(1)

# Calculate statistics by step for each phase
all_healthy_stats = all_healthy.groupby('Step')['Latency_ms'].agg(['mean', 'std', 'min', 'max'])
byz1_stats = byz1.groupby('Step')['Latency_ms'].agg(['mean', 'std', 'min', 'max'])
byz2_stats = byz2.groupby('Step')['Latency_ms'].agg(['mean', 'std', 'min', 'max'])

# Print statistics
print("=" * 70)
print("ALL HEALTHY NODES")
print("=" * 70)
print(all_healthy_stats.round(2))
print()

print("=" * 70)
print("1 BYZANTINE NODE")
print("=" * 70)
print(byz1_stats.round(2))
print()

print("=" * 70)
print("2 BYZANTINE NODES")
print("=" * 70)
print(byz2_stats.round(2))
print()

# Focus on L1 Consensus (Commit Session) - the only L1 operation
all_healthy_commit = all_healthy[all_healthy['Step'] == 'Commit Session']['Latency_ms']
byz1_commit = byz1[byz1['Step'] == 'Commit Session']['Latency_ms']
byz2_commit = byz2[byz2['Step'] == 'Commit Session']['Latency_ms']

print("=" * 70)
print("L1 CONSENSUS (Commit Session) LATENCY COMPARISON")
print("=" * 70)
print(f"All Healthy (f=0):  {all_healthy_commit.mean():.2f} ms  (baseline)")
print(f"1 Byzantine (f=1):  {byz1_commit.mean():.2f} ms  (+{((byz1_commit.mean() - all_healthy_commit.mean()) / all_healthy_commit.mean() * 100):.1f}%)")
print(f"2 Byzantine (f=2):  {byz2_commit.mean():.2f} ms  (+{((byz2_commit.mean() - all_healthy_commit.mean()) / all_healthy_commit.mean() * 100):.1f}%)")
print()

# Create single visualization - L1 Consensus Comparison
fig, ax = plt.subplots(figsize=(10, 6))

phases = ['All Healthy\n(f=0)', '1 Byzantine\n(f=1)', '2 Byzantine\n(f=2)']
commit_latencies = [all_healthy_commit.mean(), byz1_commit.mean(), byz2_commit.mean()]
colors = ['#2ecc71', '#f39c12', '#e74c3c']

bars = ax.bar(phases, commit_latencies, color=colors, alpha=0.8, edgecolor='black', linewidth=1.5)
ax.set_ylabel('L1 Consensus Latency (ms)', fontsize=13, fontweight='bold')
ax.set_title('L1 Consensus Latency - Byzantine Fault Impact', fontsize=15, fontweight='bold', pad=15)
ax.grid(axis='y', alpha=0.3, linestyle='--')

# Add value labels on bars with percentage
for i, (phase, latency) in enumerate(zip(phases, commit_latencies)):
    if i == 0:
        label = f'{latency:.1f} ms'
        y_offset = max(commit_latencies) * 0.03
    else:
        pct_increase = ((latency - commit_latencies[0]) / commit_latencies[0] * 100)
        label = f'{latency:.1f} ms (+{pct_increase:.1f}%)'
        y_offset = max(commit_latencies) * 0.03
    ax.text(i, latency + y_offset, label, 
            ha='center', va='bottom', fontsize=11, fontweight='bold')

# Ensure y-axis has enough space for labels
ax.set_ylim(0, max(commit_latencies) * 1.15)

plt.tight_layout()
output_file = 'records/l1_consensus_comparison.png'
plt.savefig(output_file, dpi=300, bbox_inches='tight')
print(f"✅ L1 Consensus comparison saved: {output_file}")
print()

print("=" * 70)
print("ANALYSIS COMPLETE")
print("=" * 70)
print(f"📁 Chart saved: {output_file}")
print(f"📁 Raw data: {latest_csv}")
print()