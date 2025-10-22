#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import glob
import re
from datetime import datetime

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

def calculate_l1_latency(csv_files):
    """
    Extract average L1 latency from each CSV.
    L1 = "Commit Session" step only
    """
    new_data = {}
    
    print("="*60)
    print("CALCULATING L1 LATENCIES")
    print("="*60)
    
    for config, filepath in csv_files.items():
        try:
            df = pd.read_csv(filepath)
            
            # Filter for "Commit Session" step only (L1 operations)
            l1_steps = df[df['Step'] == 'Commit Session']
            
            if len(l1_steps) == 0:
                print(f"{config}: WARNING - No 'Commit Session' steps found")
                continue
            
            avg_latency = l1_steps['Latency_ms'].mean()
            new_data[config] = avg_latency
            
            print(f"{config}: {avg_latency:.1f} ms (avg across {len(l1_steps)} samples)")
            
        except Exception as e:
            print(f"{config}: ERROR reading file - {e}")
    
    print("="*60)
    print()
    
    return new_data

def plot_l1_latency(data, output_prefix='l1_latency'):
    """
    Create single bar plot for L1 latency across configurations.
    """
    # Prepare data for plotting
    configs = sorted([c for c in ['4-1', '4-2', '4-3', '4-4'] if c in data],
                     key=lambda x: int(x.split('-')[1]))
    
    if not configs:
        print("ERROR: No configurations to plot!")
        return
    
    values = [data[c] for c in configs]

    # Create the figure
    fig, ax = plt.subplots(figsize=(8, 6))

    # Set up bar positions
    x = np.arange(len(configs))
    bar_width = 0.6

    # Color scheme
    bar_color = '#fc9272'  # Red/salmon for L1 operations

    # Create bars
    bars = ax.bar(x, values, bar_width, 
                  label='L1 Commit', color=bar_color, 
                  edgecolor='black', linewidth=0.8)

    # Add value labels on bars
    for bar in bars:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}', ha='center', va='bottom', 
                fontsize=11, fontweight='bold')

    # Customize plot
    ax.set_xlabel('Node Configuration (L1-L2)', fontsize=15, labelpad=10)
    ax.set_ylabel('Latency (ms)', fontsize=15, labelpad=10)
    ax.set_title('Layer 1 Commit Latency', 
                 fontsize=16, pad=15)
    ax.set_xticks(x)
    ax.set_xticklabels(configs, fontsize=13)
    ax.tick_params(axis='y', labelsize=12)

    # Add grid
    ax.yaxis.grid(True, linestyle='--', alpha=0.5, zorder=0)

    # Set y-axis limits
    ax.set_ylim(0, max(values) * 1.15)

    # Display latency values
    print("="*60)
    print("L1 COMMIT LATENCIES")
    print("="*60)
    
    for config in configs:
        latency = data[config]
        print(f"{config}: {latency:.1f} ms")
    
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
    print("L1 LATENCY PLOTTER")
    print("="*60)
    print(f"Directory: {os.path.abspath(records_dir)}")
    print(f"File prefix: {file_prefix}")
    print("="*60)
    print()
    
    # Auto-detect CSV files
    csv_files = get_csv_files(records_dir, file_prefix)
    
    # Calculate L1 latencies
    l1_data = calculate_l1_latency(csv_files)
    
    # Create plot
    plot_l1_latency(l1_data)
    
    print("Done! 🎉")

if __name__ == '__main__':
    main()