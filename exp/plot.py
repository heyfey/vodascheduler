import numpy as np
import pandas as pd
from scipy.interpolate import interpolate
import matplotlib.pyplot as plt

gpu_num = 12

# average every avg_window elements
avg_window = 12

# smooth_line_samples = 1000
smooth_line_samples = 150

# csv_list = ['ffdl_test_util.csv', 'afsl_test_util.csv', 'afsl_run_0.csv']
# label_list = ['FfDL Optimizer', 'AFS-L', 'AFS-L-0']

csv_list = ['afsl_run_0.csv', 'ffdl_run_0.csv', 'et_run_0.csv', 'efifo_run_0.csv']
csv_list = ['efifo_run1_mem.csv', 'et_run1_mem.csv', 'ffdl_run1_mem.csv', 'afsl_run1_mem.csv']
# csv_list = ['efifo_run_1.csv', 'et_run_1.csv', 'ffdl_run_1.csv', 'afsl_run_1.csv']
csv_list = ['run_2/gpu_mem_et.csv']
label_list = ['E-FIFO', 'E-Tiresias', 'FfDL', 'AFS-L']

run = 'run_1_hard'
run = 'run_0_hard'
run = 'run_0_m'
run = 'run_1_hard'
run = 'run_0_mh'
algo_list = ['efifo', 'et', 'ffdl', 'afsl']
algo_list = ['efifo', 'ffdl', 'afsl']
# algo_list = ['efifo', 'ffdl', 'afsl', 'fifo']
csv_prefix = ['gpu_mem_', 'gpu_util_']

font_size = 24
subplots = [411, 412, 413, 414]
figure = plt.figure(figsize=(20, 24))
figure = plt.figure(figsize=(20, 32))

for j, algo in enumerate(algo_list):
    print(j)
    print(algo)

    csv_list = [run+'/'+prefix+algo+'.csv' for prefix in csv_prefix]

    if algo == 'efifo':
        title = 'Elastic-FIFO'
    elif algo == 'et':
        title = 'Elastic-Tiresias'
    elif algo == 'ffdl':
        title = 'FfDL Optimizer'
    elif algo == 'afsl':
        title = 'AFS-L'
    elif algo == 'fifo':
        title = 'FIFO'
    else:
        raise('Unknown algo')

    plt.subplot(subplots[j])
    #plt.figure(figsize=(12, 8))
    #plt.figure(figsize=(20, 8))
    plt.title(title, fontsize=font_size)
    # if j == len(algo_list)-1:
    #     plt.xlabel('Time (minute)', fontsize=font_size)
    plt.xlabel('Time (minute)', fontsize=font_size)
    plt.ylabel('(%)', fontsize=font_size)

    markers = ['o', 's', 'v', '^', 'D']

    for i, csv in enumerate(csv_list):
        df = pd.read_csv(csv)

        df['Time'] -= df['Time'][0]
        df['Time'] /= 1000
        df['Time'] /= 60

        col_list = list(df)
        col_list.remove('Time')
        df['Sum'] = df[col_list].sum(axis=1) / gpu_num
        if 'mem' in csv:
            label = 'GPU memory used'
            df['Sum'] = df['Sum'] / 100
        else:
            label = 'GPU utilization'

        x = np.array(df['Time'])
        y = np.array(df['Sum'])

        avg = np.mean(df['Sum'])

        # https://www.geeksforgeeks.org/averaging-over-every-n-elements-of-a-numpy-array/
        # discard elements from the tail so the array can be reshaped
        discards = len(x) % avg_window
        if discards != 0:
            x = x[:-discards]
            y = y[:-discards]
        x = np.average(x.reshape(-1, avg_window), axis=1)
        y = np.average(y.reshape(-1, avg_window), axis=1)

        # https://www.kite.com/python/answers/how-to-plot-a-smooth-line-with-matplotlib-in-python
        x_new = np.linspace(x[0], x[-1], smooth_line_samples)
        a_BSpline = interpolate.make_interp_spline(x, y)
        y_new = a_BSpline(x_new)

        # plt.step(x_new, y_new, where='post', label=label_list[i])
        if label == 'GPU memory used':
            label = label + ', avg: ' + str(round(avg, 2))
            plt.step(x_new, y_new, label=label, marker=markers[i])
        else:
            label = label + ', avg: ' + str(round(avg, 2))
            plt.plot(x_new, y_new, label=label, marker=markers[i], linestyle='dashed')
        
    # plt.xlim([-5, 7500/60])
    plt.xlim([-5, 130])
    plt.ylim([-10, 100])
    # if j == 0:
    #     plt.legend(fontsize=font_size)
    plt.legend(fontsize=font_size)


    plt.xticks(fontsize=font_size)
    plt.yticks(fontsize=font_size)

figure.tight_layout(pad=3.0)
plt.savefig(run+'/gpu.png')
