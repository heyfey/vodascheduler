#%%
import re
import csv
import os

prefix_list = ['run_0', 'run_1', 'run_2']
prefix_list_hard = ['run_0_hard', 'run_1_hard', 'run_2_hard', 'run_3_hard']
# path = 'run_0/logs/my_file-20210721-032124.log'
# path = 'run_1/my_file-20210721-102816.log'
job_kinds = 4
results = []
field_names = [
        'run', 'algo', 'makespan_secs', 'elaspedTotalSeconds', 'gpuTotalSeconds', 'ranTotalSeconds',
        'waitedTotalSeconds', 'deleted_workers', 'deleted_launchers',
        ]

all_logs_files = []
runs = prefix_list + prefix_list_hard
runs = ['run_0_m']
runs = ['run_fifo_3node']
output_file = 'results_run_fifo_3node.csv'
for folder in runs:
    files = os.listdir(folder)
    for f in files:
        if os.path.splitext(f)[-1] == '.log':
            all_logs_files.append(os.path.join(folder, f))
print(all_logs_files)


# I0721 09:11:40.133367    4910 scheduler.go:474]  "msg"="Training job completed"  "elaspedTotalSeconds"=899.169652972 "gpuTotalSeconds"=1740.047655167 "job"="tf2-keras-cifar10-resnet50-elastic-20210721-085639" "ranTotalSeconds"=810.022391867 "scheduler"="default" "waitedTotalSeconds"=89.147261105
global deleted_workers
global deleted_launchers

#%%
def parse(word, d):
    if '\"job\"' in word:
        d["job"] = word[-1].replace("\"", "")
    elif '\"elaspedTotalSeconds\"' in word:
        d['elaspedTotalSeconds'] = float(word[-1])
    elif '\"waitedTotalSeconds\"' in word:
        d['waitedTotalSeconds'] = float(word[-1])
    elif '\"ranTotalSeconds\"' in word:
        d['ranTotalSeconds'] = float(word[-1])
    elif '\"gpuTotalSeconds\"' in word:
        d['gpuTotalSeconds'] = float(word[-1])
    return d

def cal_deleted(word):
    global deleted_workers
    global deleted_launchers
    if '\"workersToDelete\"' in word:
        deleted_workers += int(word[-1])
    elif '\"launchersToDelete\"' in word:
        deleted_launchers += int(word[-1])


#%%

for path in all_logs_files[0:]:

    deleted_workers = 0
    deleted_launchers = 0

    lines = []
    ds = []
    found_first_created = False
    start_time = '' # timestamp of first job created
    end_time = '' # timestamp of last job finished
    algo = ''
    with open(path, 'r') as f:
        log = f.readlines()
        for line in log:
            lines = line.split()
            # print(lines)
            if 'completed\"' in lines:
                end_time = lines[1]
                # print(lines)
                d = dict()
                for word in lines:
                    words = word.split("=")
                    # print(words)
                    d = parse(words, d)
                ds.append(d)
            elif 'created\"' in lines and not found_first_created:
                start_time = lines[1]
                found_first_created = True
            elif '\"msg\"=\"podNodeName' in lines:
                # print(lines)
                for word in lines:
                    words = word.split("=")
                    cal_deleted(words)
            elif '\"algorithm\"=\"ElasticFIFO\"' in lines:
                algo = 'Elastic-FIFO'
                # for word in lines:
                #     if '\"result\"' in word:
                #         words = word.split('=')
                #         words = words.split(',')
                #         words = words.split(':')
                #         print(words)
            elif '\"algorithm\"=\"ElasticTiresias\"' in lines:
                algo = 'Elastic-Tiresias'
            elif '\"algorithm\"=\"FfDLOptimizer\"' in lines:
                algo = 'FfDL Optimizer'
            elif '\"algorithm\"=\"AFS-L\"' in lines:
                algo = 'AFS-L'
            elif '\"algorithm\"=\"FIFO\"' in lines:
                algo = 'FIFO'



    # print(ds)

    result = {}
    for d in ds:
        for k in d.keys():
            if k != "job":
                result[k] = result.get(k, 0) + d[k]
    

    print("result dictionary : ", str(result))

    stat = result.copy()
    for k in stat.keys():
        print("ds length:")
        print(len(ds))
        stat[k] /= len(ds)

    print("stat dictionary (avg): ", str(stat))


    print("start: {}".format(start_time))
    print("end: {}".format(end_time))


    def convert_timestamp_to_secs(timestamp):
        # convert timestamp hh:mm:ss to secs
        timestamp = timestamp[:8]
        return sum(int(x) * 60 ** i for i, x in enumerate(reversed(timestamp.split(':'))))

    makespan_sec = convert_timestamp_to_secs(end_time) - convert_timestamp_to_secs(start_time)
    if makespan_sec < 0:
        makespan_sec += 86400 # shift 1 day (86400 sec) 
    print("makespan_sec: ", str(makespan_sec))

    per_job_result = {}
    for d in ds:
        job = d['job']
        job = re.sub(r"-\d{8}-\d{6}$", "", job)
        if job not in per_job_result:
            empty_dict = {}
            empty_dict['elaspedTotalSeconds'] = d['elaspedTotalSeconds']
            empty_dict['waitedTotalSeconds'] = d['waitedTotalSeconds']
            empty_dict['ranTotalSeconds'] = d['ranTotalSeconds']
            empty_dict['gpuTotalSeconds'] = d['gpuTotalSeconds']
            per_job_result[job] = empty_dict

        else:
            per_job_result[job]['elaspedTotalSeconds'] += d['elaspedTotalSeconds']
            per_job_result[job]['waitedTotalSeconds'] += d['waitedTotalSeconds']
            per_job_result[job]['ranTotalSeconds'] += d['ranTotalSeconds']
            per_job_result[job]['gpuTotalSeconds'] += d['gpuTotalSeconds']

    print("per_job_result: ", str(per_job_result))


    per_job_avg = per_job_result.copy()
    for k in per_job_avg.keys():
        for kk in per_job_avg[k].keys():
            if k == 'tf2-keras-transformer-elastic':
                # per_job_avg[k][kk] /= (len(ds)/job_kinds)
                per_job_avg[k][kk] /= 8
            else:
                # per_job_avg[k][kk] /= (len(ds)/job_kinds)
                per_job_avg[k][kk] /= 4

    print("per_job_avg: ", str(per_job_avg))

    print("deleted_workers: ", str(deleted_workers))
    print("deleted_launchers: ", str(deleted_launchers))

    print("algo: ", algo)

    # %%

    row = stat.copy()
    row['run'] = os.path.splitext(path)[0]
    row['algo'] = algo
    row['makespan_secs'] = makespan_sec
    row['deleted_workers'] = deleted_workers
    row['deleted_launchers'] = deleted_launchers
    for k in per_job_avg.keys():
        for kk in per_job_avg[k].keys():
            key = k+'-'+kk
            row[key] = per_job_avg[k][kk]
            if (key) not in field_names:
                field_names.append(key)

    results.append(row)


with open(output_file, 'w') as csvfile:
    writer = csv.DictWriter(csvfile, fieldnames=field_names)
    writer.writeheader()
    writer.writerows(results)
# %%
