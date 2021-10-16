import os
import csv

from pymongo import MongoClient

host = '10.97.148.28'
port = '27017'
_db = 'job_info'
_collection = 'tensorflow2-keras-mnist-elastic'
name = 'tensorflow2-keras-mnist-elastic'
# _collection = 'tf2-keras-cifar10-resnet50-elastic'
# name = 'tf2-keras-cifar10-resnet50-elastic'


client = MongoClient('mongodb://{}:{}'.format(host, port))

try:
    server_info = client.server_info() # Forces a call.
    # print(server_info)
except ServerSelectionTimeoutError:
    print("[ERROR]: Connection to MongoDB failed.")

db = client[_db]
collection = db[_collection]

info = {'total_epochs': 300}
step_time_sec = {
    '0': 0,
    '1': 1,
    '2': 2,
    '3': 3,
    '4': 4,
    '5': 5,
    '6': 6,
    '7': 7,
    '8': 8,
    '9': 9,
}

epoch_time_sec = {
    '0': 0,
    '1': 1,
    '2': 2,
    '3': 3,
    '4': 4,
    '5': 5,
    '6': 6,
    '7': 7,
    '8': 8,
    '9': 9,
}

speedup = {
    '0': 0,
    '1': 1,
    '2': 2,
    '3': 3,
    '4': 4,
    '5': 5,
    '6': 6,
    '7': 7,
    '8': 8,
    '9': 9,
}

efficiency = {
    '0': 0,
    '1': 1,
    '2': 1,
    '3': 1,
    '4': 1,
    '5': 1,
    '6': 1,
    '7': 1,
    '8': 1,
    '9': 1,
}

for key, value in step_time_sec.items():
    info.update({'step_time_sec.{}'.format(key): value})

for key, value in epoch_time_sec.items():
    info.update({'epoch_time_sec.{}'.format(key): value})

for key, value in speedup.items():
    info.update({'speedup.{}'.format(key): value})

for key, value in efficiency.items():
    info.update({'efficiency.{}'.format(key): value})

current_epoch = 0
remainning_epochs = 100
estimated_remainning_time_sec = 1000
elasped_time_sec = 0
running_time_sec = 0
waiting_time_sec = 0
gpu_time_sec = 0

# priority = 

info.update({
    "current_epoch": current_epoch,
    "remainning_epochs": remainning_epochs,
    "estimated_remainning_time_sec": estimated_remainning_time_sec,
    "running_time_sec": running_time_sec,
    "waiting_time_sec": waiting_time_sec,
    "gpu_time_sec": gpu_time_sec,
    "elasped_time_sec": elasped_time_sec,
})

print("db: {}, collection: {}, name: {}".format(_db, _collection, name))
print(info)
# collection.update_one({"name": name}, {"$set": info})

try:
    collection.insert_one({"name": name})
    result = collection.update_one({"name": name}, {"$set": info})
    # result = collection.insert_one(info)
    print("succeed.")
    print(result.matched_count)
except pymongo.errors.PyMongoError as e:
    print("failed.")