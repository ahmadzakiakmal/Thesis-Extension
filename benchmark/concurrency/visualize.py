import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import glob
import os

def load_concurrency_results():
    """Load all concurrency benchmark CSV files from records directory"""
    files = glob.glob("records/concurrency_*.csv")
    
    if not files:
        print("No concurrency benchmark files found in records/")
        return None
    
    dataframes = []
    for file in files:
        df = pd.read_csv(file)
        dataframes.append(df)
    
    # Combine all results
    combined = pd.concat(dataframes, ignore_index=True)
    return combined

def plot_tps_by_workers(df):
    """Plot Throughput (TPS) by number of workers"""
    plt.figure(figsize=(10, 6))
    
    # Group by workers and calculate mean TPS
    grouped = df.groupby('Workers')['TPS'].agg(['mean', 'std', 'count'])
    
    workers = grouped.index
    tps_mean = grouped['mean']
    tps_std = grouped['std'].fillna(0)
    
    plt.bar(workers, tps_mean, yerr=tps_std, capsize=5, alpha=0.7, color='#2171b5')
    plt.xlabel('Number of Workers', fontsize=12)
    plt.ylabel('Throughput (TPS)', fontsize=12)
    plt.title('Throughput vs Concurrent Workers', fontsize=14, fontweight='bold')
    plt.grid(axis='y', alpha=0.3)
    
    # Add value labels on bars
    for i, (w, tps) in enumerate(zip(workers, tps_mean)):
        plt.text(w, tps + tps_std.iloc[i], f'{tps:.1f}', 
                ha='center', va='bottom', fontsize=10)
    
    plt.tight_layout()
    plt.savefig('records/tps_by_workers.png', dpi=300)
    print("✓ Saved: records/tps_by_workers.png")
    plt.close()

def plot_latency_by_workers(df):
    """Plot Average Latency by number of workers"""
    plt.figure(figsize=(10, 6))
    
    # Group by workers
    grouped = df.groupby('Workers')[['Avg_Latency_ms', 'Min_Latency_ms', 'Max_Latency_ms']].mean()
    
    workers = grouped.index
    x = np.arange(len(workers))
    width = 0.25
    
    plt.bar(x - width, grouped['Min_Latency_ms'], width, label='Min', alpha=0.8, color='#2ca02c')
    plt.bar(x, grouped['Avg_Latency_ms'], width, label='Avg', alpha=0.8, color='#1f77b4')
    plt.bar(x + width, grouped['Max_Latency_ms'], width, label='Max', alpha=0.8, color='#d62728')
    
    plt.xlabel('Number of Workers', fontsize=12)
    plt.ylabel('Latency (ms)', fontsize=12)
    plt.title('Latency Distribution by Concurrent Workers', fontsize=14, fontweight='bold')
    plt.xticks(x, workers)
    plt.legend()
    plt.grid(axis='y', alpha=0.3)
    
    plt.tight_layout()
    plt.savefig('records/latency_by_workers.png', dpi=300)
    print("✓ Saved: records/latency_by_workers.png")
    plt.close()

def plot_success_rate(df):
    """Plot Success Rate by number of workers"""
    plt.figure(figsize=(10, 6))
    
    # Calculate success rate
    df['Success_Rate'] = (df['Successful'] / df['Total_Requests']) * 100
    
    grouped = df.groupby('Workers')['Success_Rate'].agg(['mean', 'std'])
    
    workers = grouped.index
    success_rate_mean = grouped['mean']
    success_rate_std = grouped['std'].fillna(0)
    
    plt.bar(workers, success_rate_mean, yerr=success_rate_std, capsize=5, 
            alpha=0.7, color='#2ca02c')
    plt.xlabel('Number of Workers', fontsize=12)
    plt.ylabel('Success Rate (%)', fontsize=12)
    plt.title('Request Success Rate vs Concurrent Workers', fontsize=14, fontweight='bold')
    plt.ylim([0, 105])
    plt.grid(axis='y', alpha=0.3)
    
    # Add value labels
    for w, rate in zip(workers, success_rate_mean):
        plt.text(w, rate + 1, f'{rate:.1f}%', ha='center', va='bottom', fontsize=10)
    
    plt.tight_layout()
    plt.savefig('records/success_rate_by_workers.png', dpi=300)
    print("✓ Saved: records/success_rate_by_workers.png")
    plt.close()

def plot_scalability(df):
    """Plot system scalability - TPS and Latency on same chart"""
    fig, ax1 = plt.subplots(figsize=(12, 6))
    
    grouped = df.groupby('Workers').agg({
        'TPS': 'mean',
        'Avg_Latency_ms': 'mean'
    })
    
    workers = grouped.index
    
    # Plot TPS on left y-axis
    color1 = '#2171b5'
    ax1.set_xlabel('Number of Workers', fontsize=12)
    ax1.set_ylabel('Throughput (TPS)', color=color1, fontsize=12)
    ax1.plot(workers, grouped['TPS'], marker='o', linewidth=2, 
            markersize=8, color=color1, label='TPS')
    ax1.tick_params(axis='y', labelcolor=color1)
    ax1.grid(alpha=0.3)
    
    # Plot Latency on right y-axis
    ax2 = ax1.twinx()
    color2 = '#d62728'
    ax2.set_ylabel('Average Latency (ms)', color=color2, fontsize=12)
    ax2.plot(workers, grouped['Avg_Latency_ms'], marker='s', linewidth=2, 
            markersize=8, color=color2, label='Avg Latency')
    ax2.tick_params(axis='y', labelcolor=color2)
    
    plt.title('System Scalability: Throughput vs Latency', fontsize=14, fontweight='bold')
    
    # Add legends
    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc='upper left')
    
    fig.tight_layout()
    plt.savefig('records/scalability.png', dpi=300)
    print("✓ Saved: records/scalability.png")
    plt.close()

def generate_summary_table(df):
    """Generate summary statistics table"""
    summary = df.groupby(['L1_Nodes', 'L2_Nodes', 'Workers']).agg({
        'Total_Requests': 'mean',
        'TPS': 'mean',
        'Avg_Latency_ms': 'mean',
        'Successful': 'mean'
    }).round(2)
    
    # Calculate success rate
    summary['Success_Rate_%'] = (summary['Successful'] / summary['Total_Requests'] * 100).round(2)
    
    print("\n" + "="*80)
    print("CONCURRENCY BENCHMARK SUMMARY")
    print("="*80)
    print(summary.to_string())
    print("="*80 + "\n")
    
    # Save to CSV
    summary.to_csv('records/concurrency_summary.csv')
    print("✓ Saved: records/concurrency_summary.csv")

def main():
    print("\n📊 Concurrency Benchmark Visualization")
    print("="*50)
    
    # Load data
    df = load_concurrency_results()
    if df is None:
        return
    
    print(f"\nLoaded {len(df)} benchmark results")
    print(f"Configurations tested: {df['Workers'].nunique()} different worker counts")
    
    # Generate visualizations
    print("\nGenerating visualizations...")
    plot_tps_by_workers(df)
    plot_latency_by_workers(df)
    plot_success_rate(df)
    plot_scalability(df)
    
    # Generate summary
    generate_summary_table(df)
    
    print("\n✅ Visualization complete!")
    print("Check the 'records/' directory for output files.")

if __name__ == "__main__":
    main()