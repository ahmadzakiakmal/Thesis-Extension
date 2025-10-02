#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import sys
import os

def visualize_latency(csv_file):
    # Read the CSV
    df = pd.read_csv(csv_file)
    
    # Filter out the "Complete Workflow" step for cleaner visualization
    df = df[df['Step'] != 'Complete Workflow']
    
    # Define the order of steps
    step_order = [
        'Start Session',
        'Scan Package',
        'Validate Package',
        'Quality Check',
        'Label Package',
        'Commit Session'
    ]
    
    # Create categorical type with our order
    df['Step'] = pd.Categorical(df['Step'], categories=step_order, ordered=True)
    df = df.sort_values('Step')
    
    # Create figure
    fig, ax = plt.subplots(figsize=(10, 6))
    
    # Create boxplot
    bp = df.boxplot(column='Latency_ms', by='Step', ax=ax, 
                     grid=True, patch_artist=True)
    
    # Customize
    ax.set_xlabel('Workflow Step', fontsize=12)
    ax.set_ylabel('Latency (ms)', fontsize=12)
    ax.set_title('')
    plt.suptitle('')
    
    # Rotate x labels
    plt.xticks(rotation=30, ha='right')
    
    # Color the boxes
    for patch in bp.findobj(plt.matplotlib.patches.PathPatch):
        patch.set_facecolor('#c6dbef')
        patch.set_edgecolor('black')
    
    # Add summary statistics
    print("\n" + "="*50)
    print("LATENCY SUMMARY STATISTICS")
    print("="*50)
    summary = df.groupby('Step')['Latency_ms'].agg(['mean', 'median', 'std', 'min', 'max'])
    print(summary.round(2))
    print("="*50 + "\n")
    
    # Tight layout
    plt.tight_layout()
    
    # Save figure
    output_file = csv_file.replace('.csv', '.png')
    plt.savefig(output_file, dpi=150, bbox_inches='tight')
    print(f"✓ Figure saved: {output_file}")
    
    # Show plot
    plt.show()

if __name__ == "__main__":
    if len(sys.argv) < 2:
        # Find the most recent CSV file
        records_dir = "./records"
        if not os.path.exists(records_dir):
            print("Error: No records directory found")
            sys.exit(1)
        
        csv_files = [f for f in os.listdir(records_dir) if f.endswith('.csv')]
        if not csv_files:
            print("Error: No CSV files found in records/")
            sys.exit(1)
        
        csv_files.sort()
        csv_file = os.path.join(records_dir, csv_files[-1])
        print(f"Using latest file: {csv_file}\n")
    else:
        csv_file = sys.argv[1]
    
    if not os.path.exists(csv_file):
        print(f"Error: File not found: {csv_file}")
        sys.exit(1)
    
    visualize_latency(csv_file)
