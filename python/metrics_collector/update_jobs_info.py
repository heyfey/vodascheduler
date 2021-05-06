from metrics_collector import metrics_collector

# TODO: get running mpijob (form namespace: celeste) using k8s api
running_jobs = ['tensorflow2-keras-mnist-elastic']
db_name = 'job_info'
collector = metrics_collector(running_jobs, db_name)
collector.update_info_all()
