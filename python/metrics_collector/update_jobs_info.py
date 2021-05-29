from metrics_collector import metrics_collector

db_job_info = 'job_info'
db_runnings = 'runnings'
collector = metrics_collector(db_job_info, db_runnings)
collector.update_info_all()
