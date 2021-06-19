import tensorflow as tf
from tensorflow.keras.callbacks import Callback

import collections
import csv
import io
import datetime
import os

import numpy as np
import six

from tensorflow.python.keras.utils.io_utils import path_to_string
from tensorflow.python.lib.io import file_io
from tensorflow.python.util.compat import collections_abc

# Celeste: set worker size dependent params in logger
def set_logger_params(logger, batch_size=None, workers=1):
  params = logger.params or {}
  params['workers'] = workers
  if batch_size:
    params['local_batch_size'] = batch_size
    params['global_batch_size'] = batch_size * workers
  logger.set_params(params)

# TODO(heyfey): rename this class becasue it does more than logging
class MetricsCSVLogger(Callback):
  """Callback that streams epoch results to a CSV file.
  Supports all values that can be represented as a string,
  including 1D iterables such as `np.ndarray`.
  Example:
  ```python
  csv_logger = CSVLogger('training.log')
  model.fit(X_train, Y_train, callbacks=[csv_logger])
  ```
  Arguments:
      filename: Filename of the CSV file, e.g. `'run/log.csv'`.
      separator: String used to separate elements in the CSV file.
      append: Boolean. True: append if file exists (useful for continuing
          training). False: overwrite existing file.
      total_epochs: Integer. Total training epochs.
      verbose: 0 or 1. Verbosity mode. 0 = silent, 1 = one line per epoch.
  """

  def __init__(self, filename, separator=',', append=False, 
               total_epochs=0, verbose=1):
    self.sep = separator
    self.filename = path_to_string(filename)
    self.append = append
    self.writer = None
    self.keys = None
    self.append_header = True

    self.verbose = verbose
    self.epoch = 0
    self.total_epochs = total_epochs

    # Get latest epoch from .csv, and track the epoch by increasing self.epoch 
    # by 1 in on_epoch_end().
    # Should make sure csv empty when training from scratch.
    if os.path.isfile(self.filename):
      with open(self.filename, mode='r') as f:
        rows = csv.DictReader(f)
        for row in rows:
          self.epoch = int(row['epoch']) + 1
    
    self.params = {}
    if six.PY2:
      self.file_flags = 'b'
      self._open_args = {}
    else:
      self.file_flags = ''
      self._open_args = {'newline': '\n'}
    super(MetricsCSVLogger, self).__init__()

  def set_params(self, params):
      self.params.update(params)

  def on_train_begin(self, logs=None):
    # Reset writer
    self.writer = None

    if self.append:
      if file_io.file_exists_v2(self.filename):
        with open(self.filename, 'r' + self.file_flags) as f:
          self.append_header = not bool(len(f.readline()))
      mode = 'a'
    else:
      mode = 'w'
    self.csv_file = io.open(self.filename,
                            mode + self.file_flags,
                            **self._open_args)

    # Sync self.epoch with the csv file, we have ensured the file exist.
    with open(self.filename, mode='r') as f:
      rows = csv.DictReader(f)
      for row in rows:
        self.epoch = int(row['epoch']) + 1

  def on_epoch_begin(self, epoch, logs=None):
    self.epoch_start_time = datetime.datetime.now()
    self.params['start_time'] = str(self.epoch_start_time)
    
  def on_epoch_end(self, epoch, logs=None):
    logs = logs or {}
    
    # Add params to logs
    logs.update(self.params)
    epoch_time = datetime.datetime.now() - self.epoch_start_time
    logs['epoch_time_sec'] = epoch_time.total_seconds()
    logs['step_time_sec'] = epoch_time.total_seconds() / logs['steps']
    logs['total_epochs'] = self.total_epochs

    if self.verbose:
      print("[INFO] epoch: {}/{}".format(self.epoch, self.total_epochs))
      print("[INFO] logger logs:")
      print(logs)

    def handle_value(k):
      is_zero_dim_ndarray = isinstance(k, np.ndarray) and k.ndim == 0
      if isinstance(k, six.string_types):
        return k
      elif isinstance(k, collections_abc.Iterable) and not is_zero_dim_ndarray:
        return '"[%s]"' % (', '.join(map(str, k)))
      else:
        return k

    if self.keys is None:
      self.keys = sorted(logs.keys())

    if self.model.stop_training:
      # We set NA so that csv parsers do not fail for this last epoch.
      logs = dict((k, logs[k]) if k in logs else (k, 'NA') for k in self.keys)
    
    if not self.writer:

      class CustomDialect(csv.excel):
        delimiter = self.sep

      fieldnames = ['epoch'] + self.keys

      self.writer = csv.DictWriter(
          self.csv_file,
          fieldnames=fieldnames,
          dialect=CustomDialect)
      if self.append_header:
        self.writer.writeheader()

    row_dict = collections.OrderedDict({'epoch': self.epoch})
    row_dict.update((key, handle_value(logs[key])) for key in self.keys)
    self.writer.writerow(row_dict)
    self.csv_file.flush()

    self.epoch = self.epoch + 1

  def on_train_end(self, logs=None):
    self.csv_file.close()
    self.writer = None