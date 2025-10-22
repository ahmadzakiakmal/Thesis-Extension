#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import glob
import re
from datetime import datetime

# Baseline data from the old system (with consensus between L2 nodes)
baseline_data = {
    '4-1': 58.3,
    '4-2': 203.1,
    '4-3': 221.1,
    '4-4': 227.0
}

def find_latest_csv_for_config(records_dir, l1_nodes, l2_nodes, prefix='latency'):
    """
    Find the most recent CSV file for a given configuration.
    Pattern: {prefix}_YYYY-MM-DD_HH-MM-SS_n{iterations}_l1-{l1}_l2-{l2}.csv
    """
    pattern = f"{prefix}_*_n*_l1-{l1_nodes}_l2-{l2_nodes}.csv"
    files = glob.glob(os.path.join(records_dir, pattern))
    
    if not files:
        return None
    
    # Extract timestamps and find the latest
    file_data = []
    for filepath in files:
        filename = os.path.basename(filepath)
        # Extract timestamp: latency_2025-10-08_18-29-47_...
        match = re.search(r'(\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2})', filename)
        if match:
            timestamp_str = match.group(1)
            try:
                timestamp = datetime.strptime(timestamp_str, '%Y-%m-%d_%H-%M-%S')
                file_data.append((timestamp, filepath))
            except ValueError:
                continue
    
    if not file_data:
        return None
    
    # Sort by timestamp (most recent first) and return the latest file
    file_data.sort(reverse=True, key=lambda x: x[0])
    return file_data[0][1]

def get_csv_files(records_dir='./', prefix='latency'):
    """
    Automatically find the latest CSV files for each L2 configuration.
    """
    configs = {
        '4-1': (4, 1),
        '4-2': (4, 2),
        '4-3': (4, 3),
        '4-4': (4, 4)
    }
    
    csv_files = {}
    print("="*60)
    print("AUTO-DETECTING CSV FILES")
    print("="*60)
    
    for config, (l1, l2) in configs.items():
        filepath = find_latest_csv_for_config(records_dir, l1, l2, prefix)
        if filepath:
            csv_files[config] = filepath
            filename = os.path.basename(filepath)
            print(f"{config}: ✓ Found {filename}")
        else:
            print(f"{config}: ✗ No file found for L1={l1}, L2={l2}")
    
    print("="*60)
    print()
    
    if not csv_files:
        print("ERROR: No CSV files found!")
        print(f"Looking for files matching pattern: {prefix}_*_n*_l1-*_l2-*.csv")
        print(f"In directory: {os.path.abspath(records_dir)}")
        exit(1)
    
    return csv_files

def calculate_l2_latency(csv_files):
    """
    Extract average L2 latency from each CSV.
    L2 = all steps EXCEPT "Commit Session" (L1) and "Complete Workflow" (total)
    """
    new_data = {}
    
    print("="*60)
    print("CALCULATING L2 LATENCIES")
    print("="*60)
    
    for config, filepath in csv_files.items():
        try:
            df = pd.read_csv(filepath)
            
            # Filter out "Commit Session" and "Complete Workflow" to get only L2 steps
            l2_steps = df[(df['Step'] != 'Commit Session') & 
                         (df['Step'] != 'Complete Workflow')]
            
            avg_latency = l2_steps['Latency_ms'].mean()
            new_data[config] = avg_latency
            
            print(f"{config}: {avg_latency:.1f} ms (avg across {len(l2_steps)} samples)")
            
        except Exception as e:
            print(f"{config}: ERROR reading file - {e}")
    
    print("="*60)
    print()
    
    return new_data

def plot_comparison(baseline_data, new_data, output_prefix='l2_latency_comparison'):
    """
    Create comparison plot between baseline and new architecture.
    """
    # Prepare data for plotting (only configs that exist in both datasets)
    configs = [c for c in ['4-1', '4-2', '4-3', '4-4'] 
               if c in baseline_data and c in new_data]
    
    if not configs:
        print("ERROR: No configurations to plot!")
        return
    
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

    # Calculate and display improvements
    print("="*60)
    print("LATENCY IMPROVEMENT (Sharded vs Baseline)")
    print("="*60)
    
    for config in configs:
        old = baseline_data[config]
        new = new_data[config]
        improvement = ((old - new) / old) * 100
        status = 'FASTER' if improvement > 0 else 'SLOWER'
        symbol = '↓' if improvement > 0 else '↑'
        
        print(f"{config}: {improvement:+.1f}% {symbol} {status}")
        print(f"       {old:.1f} ms → {new:.1f} ms (Δ {new - old:+.1f} ms)")
    
    print("="*60)
    print()

    # Save figure
    plt.tight_layout()
    
    png_file = f'{output_prefix}.png'
    pdf_file = f'{output_prefix}.pdf'
    
    plt.savefig(png_file, dpi=300, bbox_inches='tight')
    plt.savefig(pdf_file, bbox_inches='tight')
    
    print("✓ Figures saved:")
    print(f"  - {png_file}")
    print(f"  - {pdf_file}")
    print()

    plt.show()

def main():
    """
    Main function to orchestrate the plotting process.
    """
    # Configuration
    records_dir = './'
    file_prefix = 'latency'  # Change to 'same-shard_latency' or 'cross-shard_latency' if needed
    
    print("\n" + "="*60)
    print("L2 LATENCY COMPARISON PLOTTER")
    print("="*60)
    print(f"Directory: {os.path.abspath(records_dir)}")
    print(f"File prefix: {file_prefix}")
    print("="*60)
    print()
    
    # Auto-detect CSV files
    csv_files = get_csv_files(records_dir, file_prefix)
    
    # Calculate L2 latencies
    new_data = calculate_l2_latency(csv_files)
    
    # Create comparison plot
    plot_comparison(baseline_data, new_data)
    
    print("Done! 🎉")

if __name__ == '__main__':
    main()