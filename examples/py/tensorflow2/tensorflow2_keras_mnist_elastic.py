# Copyright 2021 Uber Technologies, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# ==============================================================================

import os

import tensorflow as tf
import horovod.tensorflow.keras as hvd
from callbacks import MetricsCSVLogger, set_logger_params
from argparse import ArgumentParser

parser = ArgumentParser(description='Celeste tf.keras MNIST Example')
parser.add_argument("--name"              , dest ="job_name", type=str, default='', help="unique name to identify training job (default: file name)")
parser.add_argument("--data-dir"          , dest ="d_dir", type=str, default='./', help="path to dataset")
parser.add_argument("--store-dir"         , dest ="s_dir", type=str, default='./', help="path to outputs (checkpoint)")
parser.add_argument("--metrics-dir"       , dest ="m_dir", type=str, default='./', help="path to metrics")
parser.add_argument('--checkpoint-format' , dest ="checkpoint_format", type=str, default='checkpoint.h5', help='checkpoint file format')

parser.add_argument("--lr"                , dest ="lr", type=float, default=0.001, help="base learning rate for a single GPU")
parser.add_argument("--batch-size"        , dest ="batch_size", type=int, default=128, help="local batch size")
parser.add_argument("--init-epoch"        , dest ="init_epoch", type=int, default=0, help="initial epoch")
parser.add_argument("--epochs"            , dest ="epochs", type=int, default=24, help="number of epochs to train")
parser.add_argument('--fp16-allreduce'    , dest ="fp16_allreduce", action='store_true', default=False, help='use fp16 compression during allreduce')
parser.add_argument('--warmup-epochs'     , dest ="warmup_epochs", type=int, default=0, help='number of warmup epochs')
parser.add_argument('--batches-per-commit', dest ="batches_per_commit", type=int, default=0, help='commit state every # batches, default haft an epoch')

args = parser.parse_args()

# set and print hyper-parameters
job_name           = args.job_name
dataset_dir        = args.d_dir
store_dir          = args.s_dir
metrics_dir        = args.m_dir
checkpoint_format  = args.checkpoint_format
batch_size         = args.batch_size
base_lr            = args.lr
init_epoch         = args.init_epoch
epochs             = args.epochs
fp16_allreduce     = args.fp16_allreduce
warmup_epochs      = args.warmup_epochs
batches_per_commit = args.batches_per_commit

print('[TFver] tf version: {:s}'.format(tf.__version__) )
print('[param] Listing hyper-parameters...')
print('[param] job_name : {:s}'.format(job_name) )
print('[param] dataset_dir : {:s}'.format(dataset_dir) )
print('[param] store_dir : {:s}'.format(store_dir) )
print('[param] metrics_dir : {:s}'.format(metrics_dir) )
print('[param] checkpoint_format : {:s}'.format(checkpoint_format) )
print('[param] batch_size: {:d}'.format(batch_size) )
print('[param] base_lr: {:f}'.format(base_lr) )
print('[param] init_epoch: {:d}'.format(init_epoch) )
print('[param] epochs: {:d}'.format(epochs) )
print('[param] fp16-allreduce: {}'.format(fp16_allreduce) )
print('[param] warmup_epochs: {:d}'.format(warmup_epochs) )
print('[param] batches_per_commit: {:d}'.format(batches_per_commit) )

# Set job_name as filename if not specified.
if not job_name:
    job_name = os.path.basename(__file__)
    job_name = os.path.splitext(job_name)[0]
    print("[Warnning] missing job_name, using file name as job_name: {:s}".format(job_name))
metrics_path = os.path.join(metrics_dir, job_name + '.csv')

checkpoint_path = os.path.join(store_dir, checkpoint_format)

# Horovod: initialize Horovod.
hvd.init()

# Horovod: pin GPU to be used to process local rank (one GPU per process)
gpus = tf.config.experimental.list_physical_devices('GPU')
for gpu in gpus:
    tf.config.experimental.set_memory_growth(gpu, True)
if gpus:
    tf.config.experimental.set_visible_devices(gpus[hvd.local_rank()], 'GPU')

# If set > 0, will resume training from a given checkpoint.
resume = True if init_epoch > 0 else False

# Horovod: print logs on the first worker.
verbose = 2 if hvd.rank() == 0 else 0

(mnist_images, mnist_labels), _ = \
    tf.keras.datasets.mnist.load_data(path='mnist-%d.npz' % hvd.rank())

dataset = tf.data.Dataset.from_tensor_slices(
    (tf.cast(mnist_images[..., tf.newaxis] / 255.0, tf.float32),
             tf.cast(mnist_labels, tf.int64))
)

# Calulate steps_per_epoch and batches_per_commit
steps_per_epoch = int(dataset.__len__() // batch_size)
if not batches_per_commit:
    batches_per_commit = (steps_per_epoch // hvd.size()) // 2

dataset = dataset.repeat().shuffle(10000).batch(batch_size)

model = tf.keras.Sequential([
    tf.keras.layers.Conv2D(32, [3, 3], activation='relu'),
    tf.keras.layers.Conv2D(64, [3, 3], activation='relu'),
    tf.keras.layers.MaxPooling2D(pool_size=(2, 2)),
    tf.keras.layers.Dropout(0.25),
    tf.keras.layers.Flatten(),
    tf.keras.layers.Dense(128, activation='relu'),
    tf.keras.layers.Dropout(0.5),
    tf.keras.layers.Dense(10, activation='softmax')
])

# Horovod: (optional) compression algorithm.
compression = hvd.Compression.fp16 if fp16_allreduce else hvd.Compression.none

# Horovod: adjust learning rate based on number of GPUs.
scaled_lr = base_lr * hvd.size()

opt = tf.optimizers.Adam(scaled_lr)

# Horovod: add Horovod DistributedOptimizer.
opt = hvd.DistributedOptimizer(opt, 
                               backward_passes_per_step=1, 
                               average_aggregated_gradients=True, 
                               compression=compression)

# Horovod: Specify `experimental_run_tf_function=False` to ensure TensorFlow
# uses hvd.DistributedOptimizer() to compute gradients.
model.compile(loss=tf.losses.SparseCategoricalCrossentropy(),
              optimizer=opt,
              metrics=['accuracy'],
              experimental_run_tf_function=False)

# Horovod: initialize optimizer state so we can synchronize across workers
# Keras has empty optimizer variables() for TF2:
# https://sourcegraph.com/github.com/tensorflow/tensorflow@v2.4.1/-/blob/tensorflow/python/keras/optimizer_v2/optimizer_v2.py#L351:10
model.fit(dataset, steps_per_epoch=1, epochs=1, callbacks=None)

if resume:
    model.load_weights(checkpoint_path)

# TODO: Should we get init_epoch from args or csv? Currently from args.
state = hvd.elastic.KerasState(model, batch=0, epoch=init_epoch)

# This stll causees problem since the newly added workers has different 
# init_epoch with the original workers. When we accidently remove the 
# original root rank, the new root rank may has incorrect number of current epoch.
# Therefore, we track epoch in the csv in ther callback.
# Don't set append=False. Make sure empty csv when training from scratch, 
# or epoch may be overrided in the callback.
metrics_logger = MetricsCSVLogger(metrics_path, append=True, 
                                  initial_epoch=init_epoch,
                                  total_epochs=epochs)
set_logger_params(metrics_logger, batch_size=batch_size, workers=hvd.size())
checkpoint = tf.keras.callbacks.ModelCheckpoint(checkpoint_path, save_weights_only=True)

callbacks = [
    # hvd.callbacks.BroadcastGlobalVariablesCallback(0), # TODO: do I need this?
    # hvd.callbacks.LearningRateWarmupCallback(initial_lr=scaled_lr, warmup_epochs=warmup_epochs, verbose=verbose),
    # hvd.callbacks.MetricAverageCallback(), # TODO: do I need this?
    # tf.keras.callbacks.ReduceLROnPlateau(monitor="loss", patience=10, verbose=verbose),
    
    hvd.elastic.UpdateEpochStateCallback(state),
    hvd.elastic.UpdateBatchStateCallback(state),
    hvd.elastic.CommitStateCallback(state, batches_per_commit=100), # TODO: would hang when reset with variable batches_per_commit
]

# Horovod: save checkpoints only on worker 0 to prevent other workers from corrupting them.
callbacks_root = callbacks[:]
callbacks_root.extend([metrics_logger, checkpoint])

def on_state_reset():
    # Set worker-size-dependent params in logger
    set_logger_params(metrics_logger, batch_size=batch_size,
                      workers=hvd.size())
    # reset callbacks_root to add metrics_logger with correct params
    callbacks_root = callbacks[:]
    callbacks_root.extend([metrics_logger, checkpoint])

    tf.keras.backend.set_value(state.model.optimizer.lr, base_lr * hvd.size())
    # Re-initialize, to join with possible new ranks
    state.model.fit(dataset, steps_per_epoch=1, epochs=1, callbacks=None)

# These callbacks are called after horovoe has reinitialized, but before state is synchronized across the workers.
state.register_reset_callbacks([on_state_reset])

# Train the model.
# Horovod: adjust number of steps based on number of GPUs.
@hvd.elastic.run
def train(state):
    state.model.fit(dataset,
                    steps_per_epoch=steps_per_epoch // hvd.size(),
                    epochs=epochs-state.epoch,
                    callbacks=callbacks_root if hvd.rank() == 0 else callbacks,
                    verbose=0)

train(state)