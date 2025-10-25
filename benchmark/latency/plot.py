#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import glob
import argparse

# Updated baseline data from the old system (with consensus between L2 nodes)
# Format: L2_node_count -> average_latency_ms
baseline_data = {
    1: 58.3,
    2: 203.1,
    3: 221.1,
    4: 227.0,
    5: 258.2,
    6: 289.4,
    7: 320.6,
    8: 351.8,
    9: 383.0,
    10: 414.3
}

def extract_l2_count_from_filename(filename):
    """Extract L2 node count from filename pattern: latency_*_l2-X.csv"""
    parts = filename.split('_')
    for part in parts:
        if part.startswith('l2-'):
            # Remove .csv extension if present
            l2_part = part.split('-')[1]
            if l2_part.endswith('.csv'):
                l2_part = l2_part[:-4]
            return int(l2_part)
    return None

def calculate_l2_latency(filepath):
    """Calculate average L2 latency and standard deviation excluding L1 operations"""
    df = pd.read_csv(filepath)
    
    # Filter out "Commit Session" (L1) and "Complete Workflow" (total) to get only L2 steps
    l2_steps = df[(df['Step'] != 'Commit Session') & (df['Step'] != 'Complete Workflow')]
    avg_latency = l2_steps['Latency_ms'].mean()
    std_latency = l2_steps['Latency_ms'].std()
    
    return avg_latency, std_latency

def main():
    # Set up argument parser
    parser = argparse.ArgumentParser(description='Plot L2 latency comparison between sharded and consensus architectures')
    parser.add_argument('--dir', type=str, required=True,
                       help='Directory containing latency CSV files')
    args = parser.parse_args()
    
    records_dir = args.dir
    
    # Check if directory exists
    if not os.path.exists(records_dir):
        print(f"ERROR: Directory '{records_dir}' does not exist!")
        return 1
    
    # Read all CSV files and extract data
    csv_pattern = os.path.join(records_dir, 'latency_*.csv')
    csv_files = glob.glob(csv_pattern)
    
    if not csv_files:
        print(f"ERROR: No CSV files matching pattern 'latency_*.csv' found in '{records_dir}'")
        return 1
    
    print(f"Reading data from directory: {records_dir}")
    print(f"Found {len(csv_files)} CSV files:")
    for f in csv_files:
        print(f"  - {os.path.basename(f)}")
    
    # Extract new sharded L2 data
    new_data = {}
    new_std = {}
    for filepath in csv_files:
        filename = os.path.basename(filepath)
        l2_count = extract_l2_count_from_filename(filename)
        
        if l2_count is not None:
            avg_latency, std_latency = calculate_l2_latency(filepath)
            new_data[l2_count] = avg_latency
            new_std[l2_count] = std_latency
            print(f"L2-{l2_count}: Avg L2 Latency = {avg_latency:.1f} ± {std_latency:.1f} ms (excluding Commit & Complete)")
    
    if not new_data:
        print("ERROR: No valid latency data could be extracted from CSV files!")
        return 1
    
    # Sort data by L2 node count for consistent plotting
    sorted_l2_counts = sorted(new_data.keys())
    
    # Prepare data for plotting - only include L2 counts that have both baseline and new data
    common_l2_counts = [count for count in sorted_l2_counts if count in baseline_data]
    
    if not common_l2_counts:
        print("ERROR: No matching L2 node counts between baseline and new data!")
        print(f"Baseline has: {list(baseline_data.keys())}")
        print(f"New data has: {sorted_l2_counts}")
        return 1
    
    baseline_values = [baseline_data[count] for count in common_l2_counts]
    new_values = [new_data[count] for count in common_l2_counts]
    new_errors = [new_std[count] for count in common_l2_counts]
    
    print(f"\nComparing data for L2 node counts: {common_l2_counts}")
    
    # Create the comparison figure
    fig, ax = plt.subplots(figsize=(12, 8))
    
    # Set up bar positions
    x = np.arange(len(common_l2_counts))
    bar_width = 0.35
    
    # Color scheme
    baseline_color = '#fc9272'  # Red/salmon for baseline (old with consensus)
    new_color = '#c6dbef'  # Blue for new (sharded, independent L2)
    
    # Create bars with error bars
    bars1 = ax.bar(x - bar_width/2, new_values, bar_width, 
                   label='L2 Sharded (New)', color=new_color, 
                   edgecolor='black', linewidth=0.8,
                   yerr=new_errors, capsize=5, error_kw={'linewidth': 1.5, 'capthick': 1.5, 'alpha': 0.6})
    bars2 = ax.bar(x + bar_width/2, baseline_values, bar_width, 
                   label='L2 Consensus (Baseline)', color=baseline_color, 
                   edgecolor='black', linewidth=0.8)
    
    # Add value labels on bars
    for bar in bars1:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}', ha='center', va='bottom', 
                fontsize=14, fontweight='bold')
    
    for bar in bars2:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}', ha='center', va='bottom', 
                fontsize=14, fontweight='bold')
    
    # Customize plot
    ax.set_xlabel('Number of L2 Nodes', fontsize=15, labelpad=10)
    ax.set_ylabel('Average L2 Latency (ms)', fontsize=15, labelpad=10)
    # ax.set_title('Layer 2 Average Latency Comparison:\nSharded vs Consensus-Based Architecture', 
    #              fontsize=16, pad=15)
    ax.set_xticks(x)
    ax.set_xticklabels(common_l2_counts, fontsize=12)
    ax.tick_params(axis='y', labelsize=12)
    
    # Add legend
    ax.legend(fontsize=12, loc='upper left', frameon=True, 
              framealpha=0.9, edgecolor='lightgray')
    
    # Add grid
    ax.yaxis.grid(True, linestyle='--', alpha=0.5, zorder=0)
    
    # Set y-axis limits with some padding
    ax.set_ylim(0, max(max(baseline_values), max(new_values)) * 1.15)
    
    # Calculate and display improvements
    print("\n" + "="*70)
    print("LATENCY IMPROVEMENT (Sharded vs Consensus Baseline)")
    print("="*70)
    print(f"{'L2 Nodes':<8} {'Baseline':<12} {'Sharded':<12} {'Improvement':<12} {'Status'}")
    print("-" * 70)
    
    improvements = {}
    for i, l2_count in enumerate(common_l2_counts):
        baseline_val = baseline_values[i]
        new_val = new_values[i]
        improvement = ((baseline_val - new_val) / baseline_val) * 100
        improvements[l2_count] = improvement
        
        status = 'faster' if improvement > 0 else 'slower'
        print(f"{l2_count:<8} {baseline_val:<12.1f} {new_val:<12.1f} {improvement:+.1f}%{'':<6} {status}")
    
    print("="*70)
    
    # Calculate overall statistics
    avg_improvement = np.mean(list(improvements.values()))
    print(f"Average improvement: {avg_improvement:+.1f}%")
    
    if avg_improvement > 0:
        print(f"✓ Sharded L2 shows {avg_improvement:.1f}% average latency improvement")
    else:
        print(f"⚠ Sharded L2 shows {abs(avg_improvement):.1f}% average latency increase")
    
    # Save figures in the same directory as the data
    output_png = os.path.join(records_dir, 'l2_latency_comparison.png')
    output_pdf = os.path.join(records_dir, 'l2_latency_comparison.pdf')
    
    plt.tight_layout()
    plt.savefig(output_png, dpi=300, bbox_inches='tight')
    plt.savefig(output_pdf, bbox_inches='tight')
    
    print(f"\n✓ Figures saved:")
    print(f"  - {output_png}")
    print(f"  - {output_pdf}")
    
    # Show additional analysis
    print(f"\n" + "="*50)
    print("DETAILED ANALYSIS")
    print("="*50)
    print(f"Configurations compared: {len(common_l2_counts)}")
    print(f"L2 node counts: {', '.join(map(str, common_l2_counts))}")
    print(f"Best improvement: L2-{min(improvements.keys(), key=lambda k: improvements[k])} with {max(improvements.values()):+.1f}%")
    print(f"Worst performance: L2-{max(improvements.keys(), key=lambda k: improvements[k])} with {min(improvements.values()):+.1f}%")
    
    plt.show()
    
    return 0

if __name__ == "__main__":
    exit(main())