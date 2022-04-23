import os
import csv
import re

from pymongo import MongoClient, errors
import pandas as pd
from datetime import datetime

class metrics_collector():
    def __init__(self, db_job_info, db_runnings):
        """
        Arguments:
            db_job_info: String. Name of database of job info.
            db_runnings: String. Name of database of running jobs.
        """
        self.db_name = db_job_info
        self.metrics_dir = '/metrics'

        # Service need to exist before this pod. 
        host = os.environ.get('MONGODB_SVC_SERVICE_HOST')
        port = os.environ.get('MONGODB_SVC_SERVICE_PORT')
        self.client = MongoClient('mongodb://{}:{}'.format(host, port))

        # Test connection.
        # https://stackoverflow.com/questions/30539183/how-do-you-check-if-the-client-for-a-mongodb-instance-is-valid
        try:
            info = self.client.server_info() # Forces a call.
        except ServerSelectionTimeoutError:
            print("[ERROR]: Connection to MongoDB failed.")

        self.db = self.client[self.db_name]

        # get all running jobs from db
        self.jobs = []
        scheduler_collections = self.client[db_runnings].list_collection_names()
        for collection in scheduler_collections:
            for entry in self.client[db_runnings][collection].find():
                self.jobs.append(entry['name'])
        # self.jobs.append('tensorflow2-keras-mnist-elastic') # TODO: remove this line of testing
        # self.jobs.append('tensorflow2-keras-mnist-elastic-20060102_030405') # TODO: remove this line of testing
        print("found running jobs:")
        print(self.jobs)

    def update_info_all(self):
        for job in self.jobs:
            self.parse_csv_and_update_db(job)

        self.client.close()

    def parse_csv_and_update_db(self, job):
        """
        Arguments:
            job: String. Job name.
        """
        print("Processing job: {}".format(job))
        job_name = job
        # remove timestamp from job_name
        # example job_name with timestamp: "a-20060102-030405"
        # https://stackoverflow.com/questions/36583329/regular-expression-remove-time-stamp-from-file-name
        collection_name = re.sub(r"-\d{8}-\d{6}$", "", job_name)
        collection = self.db[collection_name]

        metrics_path = os.path.join(self.metrics_dir, job + '.csv')
        try:
            df = pd.read_csv(metrics_path)
            print("Read csv sucessfully: {}".format(metrics_path))
        except:
            print("Failed to read csv: {}, skipping...".format(metrics_path))
            return

        post = collection.find_one({'name': job_name}) # TODO: error handling

        if post['current_epoch'] == df['epoch'].iloc[-1]:
            print("Same epoch, skipping...")
            return

        # Update dictionary of metrics according to the dataframe (from the csv file).
        step_time_sec = self._update_time_metric(df, post['step_time_sec'], 'step_time_sec')
        epoch_time_sec = self._update_time_metric(df, post['epoch_time_sec'], 'epoch_time_sec')
        speedup = self._update_speedup(epoch_time_sec, post['speedup'])
        efficiency = self._update_efficiency(speedup, post['efficiency'])

        current_epoch = df['epoch'].iloc[-1]
        remainning_epochs = ( post['total_epochs'] - df['epoch'].iloc[-1] ) - 1
        estimated_remainning_time_sec = float(epoch_time_sec['1']) * remainning_epochs
        
        start = datetime.strptime(df['start_time'][0], "%Y-%m-%d %H:%M:%S.%f")
        end = datetime.strptime(df['start_time'].iloc[-1], "%Y-%m-%d %H:%M:%S.%f")
        elasped_time_sec = (end - start).total_seconds() + df['epoch_time_sec'].iloc[-1]

        running_time_sec = sum(df['epoch_time_sec'])
        waiting_time_sec = elasped_time_sec - running_time_sec
        gpu_time_sec = sum(df['epoch_time_sec'] * df['workers'])

        # priority = 

        info = {
            "current_epoch": int(current_epoch),
            "remainning_epochs": int(remainning_epochs),
            "estimated_remainning_time_sec": float(estimated_remainning_time_sec),
            "running_time_sec": float(running_time_sec),
            "waiting_time_sec": float(waiting_time_sec),
            "gpu_time_sec": float(gpu_time_sec),
            "elasped_time_sec": float(elasped_time_sec),
        }

        dic_dict = {'step_time_sec': step_time_sec, 'epoch_time_sec': epoch_time_sec,
            'speedup': speedup, 'efficiency': efficiency}
        for name, dic in dic_dict.items():
            info.update(self._to_mongo_dict(name, dic))
        
        # Note: The record need to exist before we update it.
        try:
            result = collection.update_one({"name": job_name}, {"$set": info})
            print("Update succeeded, match count: {}".format(result.matched_count))
        except errors.PyMongoError as e:
            print("Failed.")

    def _update_time_metric(self, df, dic, metric):
        """ Update and return dictionary of step_time_sec or epoch_time_sec according to dataframe.
        Arguments:
            df: dataframe.
            dic: dictionary.
            metric: string. 'step_time_sec' or 'epoch_time_sec'.
        Return: dictionary. Updated dic.
        """
        for workers in df['workers'].unique():
            dic[str(workers)] = df.loc[df['workers'] == workers][metric].mean()
        return dic

    def _update_speedup(self, epoch_time_sec, speedup):
        """ Update and return dictionary of speedup according to epoch_time_sec.
        Arguments:
            epoch_time_sec: dictionary.
            speedup: dictionary.
        Return: dictionary. Updated speedup.
        """
        for key in epoch_time_sec:
            if key == '0':
                continue
            speedup[key] = epoch_time_sec['1'] / epoch_time_sec[key]
        return speedup

    def _update_efficiency(self, speedup, efficiency):
        """ Update and return dictionary of efficiency according to speedup.
        Arguments:
            speedup: dictionary.
            efficiency: dictionary.
        Return: dictionary. Updated efficiency.
        """
        for key in speedup:
            if key == '0':
                continue
            efficiency[key] = speedup[key] / float(key)
        return efficiency

    def _to_mongo_dict(self, name, dic):
        """ Convert a dictionary to mongo format.
            i.e. dict = {'a': '1'} -> {'dict.a': '1'}
        Arguments:
            name: string. Name of dic.
            dic: dictionary.
        Return: dictionary.
        """
        mongo_dict = {}
        for key, value in dic.items():
            mongo_dict["{}.{}".format(name, key)] = value
        return mongo_dict
