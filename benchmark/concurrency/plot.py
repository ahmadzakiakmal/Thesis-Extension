#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import glob
import re
import argparse
from datetime import datetime

def extract_config_from_filename(filename):
    """Extract L1, L2, workers, and duration from filename"""
    # Pattern: concurrency_*_w{workers}_d{duration}_l1-{l1}_l2-{l2}.csv
    match = re.search(r'w(\d+)_d(\d+)s_l1-(\d+)_l2-(\d+)', filename)
    if match:
        workers, duration, l1, l2 = map(int, match.groups())
        return workers, duration, l1, l2
    return None

def find_latest_csv_files(records_dir):
    """Find the most recent CSV files for each L2 configuration"""
    pattern = os.path.join(records_dir, 'concurrency_*.csv')
    files = glob.glob(pattern)
    
    config_files = {}
    
    for filepath in files:
        filename = os.path.basename(filepath)
        config = extract_config_from_filename(filename)
        
        if config:
            workers, duration, l1, l2 = config
            
            # Extract timestamp
            match = re.search(r'(\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2})', filename)
            if match:
                timestamp_str = match.group(1)
                try:
                    timestamp = datetime.strptime(timestamp_str, '%Y-%m-%d_%H-%M-%S')
                    
                    # Use L2 count as key, store most recent file
                    if l2 not in config_files or timestamp > config_files[l2][0]:
                        config_files[l2] = (timestamp, filepath, workers, duration, l1)
                except ValueError:
                    continue
    
    return config_files

def load_concurrency_data(records_dir):
    """Load and process concurrency benchmark data"""
    config_files = find_latest_csv_files(records_dir)
    
    data = {}
    
    for l2_count, (timestamp, filepath, workers, duration, l1) in config_files.items():
        try:
            df = pd.read_csv(filepath)
            
            if len(df) > 0:
                row = df.iloc[0]  # Should only be one row per file
                data[l2_count] = {
                    'l1_nodes': row['L1_Nodes'],
                    'l2_nodes': row['L2_Nodes'],
                    'workers': row['Workers'],
                    'duration': row['Duration_s'],
                    'total_requests': row['Total_Requests'],
                    'successful': row['Successful'],
                    'failed': row['Failed'],
                    'tps': row['TPS'],
                    'avg_latency': row['Avg_Latency_ms'],
                    'min_latency': row['Min_Latency_ms'],
                    'max_latency': row['Max_Latency_ms']
                }
                print(f"L2-{l2_count}: {row['Total_Requests']} requests, {row['TPS']:.1f} TPS, {row['Avg_Latency_ms']:.1f}ms avg latency")
        except Exception as e:
            print(f"Error loading {filepath}: {e}")
    
    return data

def create_concurrency_analysis(data, output_dir):
    """Create the 2x2 concurrency analysis plot"""
    if not data:
        print("No data to plot!")
        return
    
    # Sort by L2 node count
    l2_counts = sorted(data.keys())
    
    # Extract data for plotting
    tps_values = [data[l2]['tps'] for l2 in l2_counts]
    total_requests = [data[l2]['total_requests'] for l2 in l2_counts]
    avg_latencies = [data[l2]['avg_latency'] for l2 in l2_counts]
    min_latencies = [data[l2]['min_latency'] for l2 in l2_counts]
    max_latencies = [data[l2]['max_latency'] for l2 in l2_counts]
    success_rates = [(data[l2]['successful'] / data[l2]['total_requests']) * 100 for l2 in l2_counts]
    
    # Get common parameters for title
    workers = data[l2_counts[0]]['workers']
    l1_nodes = data[l2_counts[0]]['l1_nodes']
    
    # Create 2x2 subplot
    fig, ((ax1, ax2), (ax3, ax4)) = plt.subplots(2, 2, figsize=(15, 10))
    fig.suptitle(f'Concurrency Benchmark Analysis\n(L1={l1_nodes} nodes, Workers={workers})', 
                 fontsize=16, fontweight='bold')
    
    # 1. Throughput vs L2 Node Count (Top Left)
    bars1 = ax1.bar(l2_counts, tps_values, color='steelblue', edgecolor='black', linewidth=0.8)
    ax1.set_xlabel('Number of L2 Nodes', fontsize=12)
    ax1.set_ylabel('Throughput (TPS)', fontsize=12)
    ax1.set_title('Throughput vs L2 Node Count', fontsize=14, fontweight='bold')
    ax1.set_xticks(l2_counts)  # Force discrete x-axis
    ax1.grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, value in zip(bars1, tps_values):
        ax1.text(bar.get_x() + bar.get_width()/2., bar.get_height(),
                f'{value:.1f}', ha='center', va='bottom', 
                fontsize=11, fontweight='bold')
    
    # 2. Latency Distribution (Top Right) - Error Bar Chart
    # Calculate error values (distance from avg to min/max)
    error_low = [avg - min_val for avg, min_val in zip(avg_latencies, min_latencies)]
    error_high = [max_val - avg for avg, max_val in zip(avg_latencies, max_latencies)]
    
    # Create error bar chart
    bars2 = ax2.bar(l2_counts, avg_latencies, color='steelblue', edgecolor='black', linewidth=0.8,
                    yerr=[error_low, error_high], capsize=8, 
                    error_kw={'linewidth': 2, 'capthick': 2, 'ecolor': 'darkred'})
    
    ax2.set_xlabel('Number of L2 Nodes', fontsize=12)
    ax2.set_ylabel('Latency (ms)', fontsize=12)
    ax2.set_title('Latency Distribution (Avg ± Min/Max Range)', fontsize=14, fontweight='bold')
    ax2.set_xticks(l2_counts)
    ax2.grid(True, alpha=0.3)
    
    # Add value labels showing avg latency
    for bar, avg_val in zip(bars2, avg_latencies):
        ax2.text(bar.get_x() + bar.get_width()/2., bar.get_height(),
                f'{avg_val:.1f}', ha='center', va='bottom', 
                fontsize=11, fontweight='bold')
    
    # 3. Request Success Rate (Bottom Left)
    bars3 = ax3.bar(l2_counts, success_rates, color='lightgreen', edgecolor='black', linewidth=0.8)
    ax3.set_xlabel('Number of L2 Nodes', fontsize=12)
    ax3.set_ylabel('Success Rate (%)', fontsize=12)
    ax3.set_title('Request Success Rate', fontsize=14, fontweight='bold')
    ax3.set_ylim(0, 105)
    ax3.set_xticks(l2_counts)  # Force discrete x-axis
    ax3.grid(True, alpha=0.3)
    
    # Add percentage labels
    for bar, value in zip(bars3, success_rates):
        ax3.text(bar.get_x() + bar.get_width()/2., bar.get_height(),
                f'{value:.1f}%', ha='center', va='bottom', 
                fontsize=11, fontweight='bold')
    
    # 4. Total Requests Processed (Bottom Right) - Bar Chart
    bars4 = ax4.bar(l2_counts, total_requests, color='mediumpurple', 
                    edgecolor='black', linewidth=0.8)
    ax4.set_xlabel('Number of L2 Nodes', fontsize=12)
    ax4.set_ylabel('Total Requests', fontsize=12)
    ax4.set_title('Total Requests Processed', fontsize=14, fontweight='bold')
    ax4.set_xticks(l2_counts)  # Force discrete x-axis
    ax4.grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, value in zip(bars4, total_requests):
        ax4.text(bar.get_x() + bar.get_width()/2., bar.get_height(),
                f'{value}', ha='center', va='bottom', 
                fontsize=11, fontweight='bold')
    
    # Adjust layout
    plt.tight_layout()
    
    # Save figures
    png_file = os.path.join(output_dir, 'concurrency_analysis.png')
    pdf_file = os.path.join(output_dir, 'concurrency_analysis.pdf')
    
    plt.savefig(png_file, dpi=300, bbox_inches='tight')
    plt.savefig(pdf_file, bbox_inches='tight')
    
    print(f"\n✓ Figures saved:")
    print(f"  - {png_file}")
    print(f"  - {pdf_file}")
    
    plt.show()
    
    # Print summary statistics
    print(f"\n" + "="*60)
    print("CONCURRENCY BENCHMARK SUMMARY")
    print("="*60)
    print(f"{'L2 Nodes':<8} {'TPS':<8} {'Avg Lat':<10} {'Requests':<10} {'Success':<8}")
    print("-" * 60)
    
    for l2 in l2_counts:
        d = data[l2]
        success_rate = (d['successful'] / d['total_requests']) * 100
        print(f"{l2:<8} {d['tps']:<8.1f} {d['avg_latency']:<10.1f} {d['total_requests']:<10} {success_rate:<8.1f}%")
    
    print("="*60)
    
    # Performance analysis
    best_tps_l2 = max(l2_counts, key=lambda x: data[x]['tps'])
    worst_latency_l2 = min(l2_counts, key=lambda x: data[x]['avg_latency'])
    most_requests_l2 = max(l2_counts, key=lambda x: data[x]['total_requests'])
    
    print(f"\nPERFORMANCE HIGHLIGHTS:")
    print(f"  Best Throughput: L2-{best_tps_l2} with {data[best_tps_l2]['tps']:.1f} TPS")
    print(f"  Best Latency: L2-{worst_latency_l2} with {data[worst_latency_l2]['avg_latency']:.1f} ms")
    print(f"  Most Requests: L2-{most_requests_l2} with {data[most_requests_l2]['total_requests']} requests")
    print("="*60)

def main():
    # Set up argument parser
    parser = argparse.ArgumentParser(description='Plot concurrency benchmark analysis')
    parser.add_argument('--dir', type=str, required=True,
                       help='Directory containing concurrency benchmark CSV files')
    args = parser.parse_args()
    
    records_dir = args.dir
    
    # Check if directory exists
    if not os.path.exists(records_dir):
        print(f"❌ Directory '{records_dir}' does not exist!")
        return 1
    
    print("="*60)
    print("CONCURRENCY BENCHMARK ANALYSIS")
    print("="*60)
    print(f"Loading data from: {os.path.abspath(records_dir)}")
    print()
    
    # Load concurrency data
    print("📂 Loading concurrency benchmark data...")
    data = load_concurrency_data(records_dir)
    
    if not data:
        print(f"❌ No valid concurrency data found in directory: {records_dir}")
        return 1
    
    print(f"✓ Loaded data for {len(data)} L2 configurations")
    print()
    
    # Create analysis plot
    print("📈 Creating concurrency analysis plot...")
    create_concurrency_analysis(data, records_dir)
    
    print("\n✅ Analysis complete!")
    return 0

if __name__ == '__main__':
    exit(main())