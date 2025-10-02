#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import re
import sys

def load_csv_files(specific_file=None):
    """Load latency CSV files"""
    if specific_file:
        # Load single file
        if not os.path.exists(specific_file):
            print(f"Error: File not found: {specific_file}")
            return None
        
        match = re.search(r'l1-(\d+)', specific_file)
        if not match:
            print("Error: Cannot parse L1 node count from filename")
            return None
        
        l1_nodes = int(match.group(1))
        df = pd.read_csv(specific_file)
        df = df[df['Step'] != 'Complete Workflow']
        print(f"Loaded: {specific_file} (L1={l1_nodes} nodes)")
        return {l1_nodes: df}, specific_file
    
    # Load all files from records
    records_dir = "./records"
    if not os.path.exists(records_dir):
        print("Error: No records directory found")
        return None
    
    csv_files = [f for f in os.listdir(records_dir) if f.startswith('latency_') and f.endswith('.csv')]
    if not csv_files:
        print("Error: No latency CSV files found in records/")
        return None
    
    dataframes = {}
    for csv_file in csv_files:
        match = re.search(r'l1-(\d+)', csv_file)
        if match:
            l1_nodes = int(match.group(1))
            filepath = os.path.join(records_dir, csv_file)
            df = pd.read_csv(filepath)
            df = df[df['Step'] != 'Complete Workflow']
            dataframes[l1_nodes] = df
            print(f"Loaded: {csv_file} (L1={l1_nodes} nodes)")
    
    return dataframes, None

def create_stacked_bar(dataframes, input_file=None):
    """Create stacked bar chart"""
    
    step_order = [
        'Start Session',
        'Scan Package',
        'Validate Package',
        'Quality Check',
        'Label Package',
        'Commit Session'
    ]
    
    configs = sorted(dataframes.keys())
    avg_latencies = {}
    
    for l1_nodes in configs:
        df = dataframes[l1_nodes]
        avg_latencies[l1_nodes] = {}
        for step in step_order:
            step_data = df[df['Step'] == step]['Latency_ms']
            avg_latencies[l1_nodes][step] = step_data.mean() if len(step_data) > 0 else 0
    
    config_labels = [f"{l1}-1" for l1 in configs]
    
    fig, ax = plt.subplots(figsize=(10, 6))
    
    colors = {
        'Start Session': '#08519c',
        'Scan Package': '#2171b5',
        'Validate Package': '#4292c6',
        'Quality Check': '#6baed6',
        'Label Package': '#9ecae1',
        'Commit Session': '#fc9272'
    }
    
    bar_width = 0.6
    x_pos = np.arange(len(configs))
    bottom = np.zeros(len(configs))
    
    for step in step_order:
        values = [avg_latencies[l1][step] for l1 in configs]
        bars = ax.bar(x_pos, values, bar_width, bottom=bottom,
                     label=step, color=colors[step], edgecolor='black', linewidth=0.8)
        
        for i, (bar, val) in enumerate(zip(bars, values)):
            if val > 20:
                height = bar.get_height()
                ax.text(bar.get_x() + bar.get_width()/2., bottom[i] + height/2.,
                       f'{val:.0f}', ha='center', va='center', fontsize=9)
        
        bottom += values
    
    for i, total in enumerate(bottom):
        ax.text(i, total + 20, f'{total:.0f}', ha='center', va='bottom', 
               fontsize=11, fontweight='bold')
    
    ax.set_xlabel('Node Configuration (L1-L2)', fontsize=12)
    ax.set_ylabel('Latency (ms)', fontsize=12)
    ax.set_title('Latency Breakdown by Configuration', fontsize=14, pad=20)
    ax.set_xticks(x_pos)
    ax.set_xticklabels(config_labels)
    ax.legend(loc='upper left', fontsize=10)
    ax.grid(axis='y', alpha=0.3, linestyle='--')
    ax.set_ylim(0, max(bottom) * 1.15)
    
    plt.tight_layout()
    
    # Determine output filename
    if input_file:
        output_file = input_file.replace('.csv', '_stacked.png')
    else:
        output_file = 'records/preview_stacked_bar.png'
    
    plt.savefig(output_file, dpi=150, bbox_inches='tight')
    print(f"\n✓ Figure saved: {output_file}")
    
    # Print summary
    print("\n" + "="*60)
    print("AVERAGE LATENCY BY CONFIGURATION (ms)")
    print("="*60)
    print(f"{'Config':<10}", end='')
    for step in step_order:
        print(f"{step:<18}", end='')
    print(f"{'Total':<10}")
    print("-"*60)
    
    for l1_nodes in configs:
        print(f"{l1_nodes}-1      ", end='')
        total = 0
        for step in step_order:
            val = avg_latencies[l1_nodes][step]
            print(f"{val:<18.1f}", end='')
            total += val
        print(f"{total:<10.1f}")
    print("="*60 + "\n")
    
    plt.show()

if __name__ == "__main__":
    print("="*60)
    print("STACKED BAR CHART - LATENCY PREVIEW")
    print("="*60 + "\n")
    
    input_file = sys.argv[1] if len(sys.argv) > 1 else None
    
    result = load_csv_files(input_file)
    if result:
        dataframes, source_file = result
        print(f"\nFound {len(dataframes)} configuration(s)")
        create_stacked_bar(dataframes, source_file)
    else:
        print("No data to visualize")
