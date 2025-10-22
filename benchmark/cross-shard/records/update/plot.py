#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
import glob
import re
import argparse
from datetime import datetime

def find_latest_csv_for_config(records_dir, l1_nodes, l2_nodes, prefix='latency'):
    """
    Find the most recent CSV file for a given configuration.
    Pattern: {prefix}_*_n*_l1-{l1}_l2-{l2}.csv
    """
    pattern = f"{prefix}_*_n*_l1-{l1_nodes}_l2-{l2_nodes}.csv"
    files = glob.glob(os.path.join(records_dir, pattern))
    
    if not files:
        return None
    
    # Extract timestamps and find the latest
    file_data = []
    for filepath in files:
        filename = os.path.basename(filepath)
        # Extract timestamp
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

def get_csv_files(records_dir, prefix, configs):
    """
    Automatically find the latest CSV files for each configuration.
    """
    csv_files = {}
    
    for config, (l1, l2) in configs.items():
        filepath = find_latest_csv_for_config(records_dir, l1, l2, prefix)
        if filepath:
            csv_files[config] = filepath
        else:
            print(f"    {config}: No file found for L1={l1}, L2={l2}")
    
    return csv_files

def calculate_average_latency(csv_files, exclude_steps=None):
    """
    Calculate average latency and standard deviation from CSV files.
    
    Args:
        csv_files: Dict of config -> filepath
        exclude_steps: List of step names to exclude (e.g., ['Commit Session', 'Complete Workflow'])
    
    Returns:
        Tuple of (latency_data, std_data) - Dict of config -> average latency, Dict of config -> std deviation
    """
    if exclude_steps is None:
        exclude_steps = ['Commit Session', 'Complete Workflow']
    
    latency_data = {}
    std_data = {}
    
    for config, filepath in csv_files.items():
        try:
            df = pd.read_csv(filepath)
            
            # Filter out excluded steps
            filtered_df = df[~df['Step'].isin(exclude_steps)]
            
            if len(filtered_df) == 0:
                print(f"    {config}: WARNING - No valid data after filtering")
                continue
            
            avg_latency = filtered_df['Latency_ms'].mean()
            std_latency = filtered_df['Latency_ms'].std()
            latency_data[config] = avg_latency
            std_data[config] = std_latency
            
        except Exception as e:
            print(f"    {config}: ERROR - {e}")
    
    return latency_data, std_data

def plot_comparison(same_shard_data, cross_shard_data, same_shard_std, cross_shard_std, output_dir='.', output_prefix='shard_comparison'):
    """
    Create comparison plot between same-shard and cross-shard operations.
    """
    # Get configs that exist in both datasets and sort them properly
    configs = [c for c in same_shard_data.keys() if c in cross_shard_data.keys()]
    
    # Custom sort to ensure proper order (4-2, 4-3, ..., 4-10)
    def sort_config(config):
        parts = config.split('-')
        return (int(parts[0]), int(parts[1]))
    
    configs = sorted(configs, key=sort_config)
    
    if not configs:
        print("\n❌ ERROR: No common configurations found between datasets!")
        return
    
    same_shard_values = [same_shard_data[c] for c in configs]
    cross_shard_values = [cross_shard_data[c] for c in configs]
    same_shard_errors = [same_shard_std[c] for c in configs]
    cross_shard_errors = [cross_shard_std[c] for c in configs]

    # Create the comparison figure
    fig, ax = plt.subplots(figsize=(14, 8))

    # Set up bar positions
    x = np.arange(len(configs))
    bar_width = 0.35

    # Color scheme
    same_shard_color = '#c6dbef'  # Blue for same-shard (direct)
    cross_shard_color = '#fc9272'  # Red/salmon for cross-shard (forwarded)

    # Create bars with error bars
    bars1 = ax.bar(x - bar_width/2, same_shard_values, bar_width, 
                   label='Same-Shard (Direct)', color=same_shard_color, 
                   edgecolor='black', linewidth=0.8,
                   yerr=same_shard_errors, capsize=5, 
                   error_kw={'linewidth': 1.5, 'capthick': 1.5, 'alpha': 0.7})
    bars2 = ax.bar(x + bar_width/2, cross_shard_values, bar_width, 
                   label='Cross-Shard (Forwarded)', color=cross_shard_color, 
                   edgecolor='black', linewidth=0.8,
                   yerr=cross_shard_errors, capsize=5,
                   error_kw={'linewidth': 1.5, 'capthick': 1.5, 'alpha': 0.7})

    # Add value labels on bars
    for bar in bars1:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}', ha='center', va='bottom', 
                fontsize=10, fontweight='bold')

    for bar in bars2:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}', ha='center', va='bottom', 
                fontsize=10, fontweight='bold')

    # Customize plot
    ax.set_xlabel('Node Configuration (L1-L2)', fontsize=15, labelpad=10)
    ax.set_ylabel('Average L2 Latency (ms)', fontsize=15, labelpad=10)
    # ax.set_title('L2 Layer: Same-Shard vs Cross-Shard Average Latency', 
    #              fontsize=16, pad=15)
    ax.set_xticks(x)
    ax.set_xticklabels(configs, fontsize=11, rotation=45 if len(configs) > 6 else 0)
    ax.tick_params(axis='y', labelsize=12)

    # Add legend
    ax.legend(fontsize=12, loc='upper left', frameon=True, 
              framealpha=0.9, edgecolor='lightgray')

    # Add grid
    ax.yaxis.grid(True, linestyle='--', alpha=0.5, zorder=0)

    # Set y-axis limits (account for error bars)
    all_values = same_shard_values + cross_shard_values
    all_errors = same_shard_errors + cross_shard_errors
    max_with_error = max([v + e for v, e in zip(all_values, all_errors)])
    ax.set_ylim(0, max_with_error * 1.15)

    # Calculate and display comparison
    print("\n" + "="*70)
    print("LATENCY COMPARISON (Same-Shard vs Cross-Shard)")
    print("="*70)
    print(f"{'Config':<10} {'Same-Shard':<15} {'Cross-Shard':<15} {'Difference':<20}")
    print("-"*70)
    
    for config in configs:
        same = same_shard_data[config]
        cross = cross_shard_data[config]
        diff = cross - same
        diff_pct = (diff / same) * 100
        
        if diff > 0:
            status = f"+{diff:.1f} ms ({diff_pct:+.1f}%) SLOWER"
        else:
            status = f"{diff:.1f} ms ({diff_pct:+.1f}%) FASTER"
        
        print(f"{config:<10} {same:>10.1f} ms   {cross:>10.1f} ms   {status}")
    
    print("="*70)
    
    # Calculate overall statistics
    avg_same = np.mean(same_shard_values)
    avg_cross = np.mean(cross_shard_values)
    overall_diff = avg_cross - avg_same
    overall_diff_pct = (overall_diff / avg_same) * 100
    
    print(f"\nOVERALL AVERAGE:")
    print(f"  Same-Shard:  {avg_same:.1f} ms")
    print(f"  Cross-Shard: {avg_cross:.1f} ms")
    print(f"  Difference:  {overall_diff:+.1f} ms ({overall_diff_pct:+.1f}%)")
    print("="*70)
    print()

    # Save figure
    plt.tight_layout()
    
    png_file = os.path.join(output_dir, f'{output_prefix}.png')
    pdf_file = os.path.join(output_dir, f'{output_prefix}.pdf')
    
    plt.savefig(png_file, dpi=300, bbox_inches='tight')
    plt.savefig(pdf_file, bbox_inches='tight')
    
    print("✓ Figures saved:")
    print(f"  - {png_file}")
    print(f"  - {pdf_file}")
    print()

    plt.show()

def main():
    parser = argparse.ArgumentParser(
        description='Compare cross-shard and same-shard latency benchmarks',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Default usage (cross-shard in current dir, same-shard in specified dir)
  python compare_shards.py --same-shard ./benchmark/latency/records/08-10
  
  # Specify both directories
  python compare_shards.py \\
    --cross-shard ./benchmark/cross-shard/records/08-10 \\
    --same-shard ./benchmark/latency/records/08-10
  
  # Custom output location
  python compare_shards.py \\
    --same-shard ./benchmark/latency/records/08-10 \\
    --output ./results
        """
    )
    
    parser.add_argument(
        '--cross-shard',
        type=str,
        default='.',
        help='Directory containing cross-shard benchmark CSV files (default: current directory)'
    )
    
    parser.add_argument(
        '--same-shard',
        type=str,
        required=True,
        help='Directory containing same-shard/direct latency benchmark CSV files'
    )
    
    parser.add_argument(
        '--output',
        type=str,
        default='.',
        help='Output directory for plots (default: current directory)'
    )
    
    parser.add_argument(
        '--configs',
        type=str,
        default='4-2,4-3,4-4,4-5,4-6,4-7,4-8,4-9,4-10',
        help='Comma-separated list of configurations to compare (default: 4-2,4-3,4-4,4-5,4-6,4-7,4-8,4-9,4-10)'
    )
    
    args = parser.parse_args()
    
    # Parse configurations
    config_list = args.configs.split(',')
    configs = {}
    for config in config_list:
        parts = config.strip().split('-')
        if len(parts) == 2:
            try:
                l1, l2 = int(parts[0]), int(parts[1])
                configs[config.strip()] = (l1, l2)
            except ValueError:
                print(f"Warning: Invalid config format '{config}', skipping")
    
    if not configs:
        print("ERROR: No valid configurations specified!")
        return
    
    # Create output directory if it doesn't exist
    os.makedirs(args.output, exist_ok=True)
    
    print("\n" + "="*70)
    print("CROSS-SHARD vs SAME-SHARD LATENCY COMPARISON")
    print("="*70)
    print(f"Cross-Shard Dir: {os.path.abspath(args.cross_shard)}")
    print(f"Same-Shard Dir:  {os.path.abspath(args.same_shard)}")
    print(f"Output Dir:      {os.path.abspath(args.output)}")
    print(f"Configurations:  {', '.join(configs.keys())}")
    print("="*70)
    print()
    
    # Find cross-shard CSV files
    print("📂 Searching for CROSS-SHARD files...")
    cross_shard_files = get_csv_files(args.cross_shard, 'cross-shard_latency', configs)
    if cross_shard_files:
        print(f"   ✓ Found {len(cross_shard_files)} file(s)")
        for config, filepath in cross_shard_files.items():
            print(f"     - {config}: {os.path.basename(filepath)}")
    else:
        print("   ✗ No cross-shard files found!")
    print()
    
    # Find same-shard CSV files
    print("📂 Searching for SAME-SHARD files...")
    # Try multiple possible prefixes
    same_shard_files = get_csv_files(args.same_shard, 'same-shard_latency', configs)
    if not same_shard_files:
        same_shard_files = get_csv_files(args.same_shard, 'latency', configs)
    
    if same_shard_files:
        print(f"   ✓ Found {len(same_shard_files)} file(s)")
        for config, filepath in same_shard_files.items():
            print(f"     - {config}: {os.path.basename(filepath)}")
    else:
        print("   ✗ No same-shard files found!")
    print()
    
    # Check if we have files to compare
    if not cross_shard_files or not same_shard_files:
        print("❌ ERROR: Missing CSV files. Cannot proceed with comparison.")
        return
    
    # Calculate latencies
    print("📊 Calculating latencies...")
    cross_shard_data, cross_shard_std = calculate_average_latency(
        cross_shard_files, 
        exclude_steps=['Commit Session', 'Complete Workflow']
    )
    same_shard_data, same_shard_std = calculate_average_latency(
        same_shard_files, 
        exclude_steps=['Commit Session', 'Complete Workflow']
    )
    
    if not cross_shard_data or not same_shard_data:
        print("❌ ERROR: Failed to calculate latencies from CSV files.")
        return
    
    print(f"   ✓ Cross-Shard: {len(cross_shard_data)} config(s) processed")
    print(f"   ✓ Same-Shard:  {len(same_shard_data)} config(s) processed")
    
    # Create comparison plot
    print("\n📈 Creating comparison plot...")
    plot_comparison(same_shard_data, cross_shard_data, same_shard_std, cross_shard_std, args.output, 'shard_comparison')
    
    print("✅ Done! 🎉\n")

if __name__ == '__main__':
    main()