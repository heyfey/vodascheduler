import os
import time
import numpy as np
import json

# Average interval of Poisson distribution
avg_interval_secs = 30
num_each_job = 6

mnist = "../examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml"
caifar10_vgg16 = "../examples/yaml/tensorflow2/tf2-keras-cifar10-vgg16-elastic.yaml"
caifar10_resnet50 = "../examples/yaml/tensorflow2/tf2-keras-cifar10-resnet50-elastic.yaml"
caifar10_inceptionv3 = "../examples/yaml/tensorflow2/tf2-keras-cifar10-inceptionv3-elastic.yaml"
transformer = "../examples/yaml/tensorflow2/tf2-keras-transformer-elastic.yaml"


# job_sequence = [caifar10_vgg16, caifar10_resnet50, caifar10_inceptionv3, transformer] * num_each_job
job_sequence = [mnist, caifar10_vgg16, caifar10_resnet50, caifar10_inceptionv3] * 4
job_sequence += [transformer] * 8

np.random.shuffle(job_sequence)

intervals = np.random.poisson(lam=avg_interval_secs, size=len(job_sequence))

config = dict()
config['job_sequence'] = job_sequence
config['intervals'] = intervals.tolist()

# print("job sequence:")
# print(config['job_sequence'])
# print("arrived (seconds):")
# print(config['intervals'])

def start(config):
    print(config['intervals'])
    for i, interval in enumerate(config['intervals']):
        # print("go run ../cmd/main.go create -f {}".format(config['job_sequence'][i]))
        os.system("go run ../cmd/main.go create -f {}".format(config['job_sequence'][i]))
        time.sleep(interval)

def save_config(config, filename='config.json'):
    with open(filename, "w") as fp:
        json.dump(config, fp)

def load_config(path):
    with open(path) as json_file:
        config = json.load(json_file)
    return config


def gen_hard_intervals():
    # total 24 jobs comining in 60 minutes
    import random
    hard_arrivals = []
    hard_arrivals.append(0)
    for _ in range (5):
        hard_arrivals.append(random.randrange(0, 180))
    for _ in range (3):
        hard_arrivals.append(random.randrange(180, 1500))
    for _ in range (8):
        hard_arrivals.append(random.randrange(1500, 1740))
    for _ in range (2):
        hard_arrivals.append(random.randrange(1740, 2700))
    for _ in range (6):
        hard_arrivals.append(random.randrange(3000, 3300))
    hard_arrivals.sort()
    print(hard_arrivals)

    hard_intervals = [hard_arrivals[i] - hard_arrivals[i-1] for i in range(len(hard_arrivals)) if i > 0]
    print(hard_intervals)

    import matplotlib.pyplot as plt
    plt.hist(hard_arrivals, bins=60)
    plt.savefig("hist.png")

    return hard_intervals

# config = {}
# config = load_config('../exp/run_1/config.json')
# # config = load_config('config.json')
# arrived = 0
# arrivals = []
# for i in config['intervals']:
#     arrivals.append(arrived/60)
#     arrived += i

# import matplotlib.pyplot as plt
# fontsize = 30
# print(config['intervals'])
# print(arrivals)
# figure = plt.figure(figsize=(12, 8))
# plt.hist(arrivals, bins=60, range=(0, 60), ec='black')
# plt.ylim([0, 5])
# # plt.title("Pattern 1", fontsize=fontsize)
# plt.xlabel('Time (minute)', fontsize=fontsize)
# plt.ylabel('# of Job Arrivals', fontsize=fontsize)
# plt.xticks(fontsize = fontsize) 
# plt.yticks(fontsize = fontsize) 
# figure.tight_layout()
# plt.savefig("hist.png")

# config['intervals'] = gen_hard_intervals()
# config['job_sequence'] = job_sequence

# save_config(config, filename='config.json')
config = load_config('config.json')
start(config)