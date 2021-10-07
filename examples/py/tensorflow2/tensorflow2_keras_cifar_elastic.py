import os
import numpy as np

import tensorflow as tf
from tensorflow.keras import applications, datasets
import horovod.tensorflow.keras as hvd
from callbacks import MetricsCSVLogger, set_logger_params
from argparse import ArgumentParser

# Benchmark settings
parser = ArgumentParser(description='Voda tf.keras CIFAR Example')
parser.add_argument("--name"              , dest="job_name", type=str, default='', help="unique name to identify training job (default: file name)")
parser.add_argument("--data-dir"          , dest="d_dir", type=str, default='./', help="path to dataset")
parser.add_argument("--store-dir"         , dest="s_dir", type=str, default='./', help="path to outputs (checkpoint)")
parser.add_argument("--metrics-dir"       , dest="m_dir", type=str, default='./', help="path to metrics")
parser.add_argument('--checkpoint-format' , dest="checkpoint_format", type=str, default='checkpoint.h5', help='checkpoint file format')

parser.add_argument('--model'              , dest='model', type=str, default='ResNet50', help='model to train')
parser.add_argument('--dataset'            , dest='dataset', type=str, default='cifar10', help='cifar10 or cifar100')
parser.add_argument("--optimizer"          , dest="optimizer", type=str, default='SGD', help='optimizer')
parser.add_argument("--momentum"           , dest="momentum", type=float, default=0.0, help='momentum')
parser.add_argument("--lr"                 , dest="lr", type=float, default=0.01, help="base learning rate for a single GPU")
parser.add_argument("--batch-size"         , dest="batch_size", type=int, default=128, help="local batch size")
parser.add_argument("--epochs"             , dest="epochs", type=int, default=90, help="number of epochs to train")
parser.add_argument('--fp16-allreduce'     , dest="fp16_allreduce", action='store_true', default=False, help='use fp16 compression during allreduce')
parser.add_argument('--warmup-epochs'      , dest="warmup_epochs", type=int, default=0, help='number of warmup epochs')
parser.add_argument('--batches-per-commit' , dest="batches_per_commit", type=int, default=100, help='commit state every # batches')

args = parser.parse_args()

# set and print hyper-parameters
job_name           = args.job_name
dataset_dir        = args.d_dir
store_dir          = args.s_dir
metrics_dir        = args.m_dir
checkpoint_format  = args.checkpoint_format
model_name         = args.model
dataset            = args.dataset
optimizer          = args.optimizer
momentum           = args.momentum
batch_size         = args.batch_size
base_lr            = args.lr
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
print('[param] model : {:s}'.format(model_name) )
print('[param] dataset : {:s}'.format(dataset) )
print('[param] optimizer : {:s}'.format(optimizer) )
print('[param] batch_size: {:d}'.format(batch_size) )
print('[param] base_lr: {:f}'.format(base_lr) )
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

# Training data
if dataset == "cifar10":
    num_classes = 10
elif dataset == "cifar100":
    num_classes = 100

# Subtracting pixel mean improves accuracy.
subtract_pixel_mean = True

# Load the dataset.
(x_train, y_train), (x_test, y_test) = getattr(datasets, dataset).load_data()


# Normalize data.
x_train = x_train.astype('float32') / 255
x_test = x_test.astype('float32') / 255

# If subtract pixel mean is enabled
if subtract_pixel_mean:
    x_train_mean = np.mean(x_train, axis=0)
    x_train -= x_train_mean
    x_test -= x_train_mean

# InceptionV3 takes minimal input size of (75, 75)
if model_name == "InceptionV3":
    x_train = tf.image.resize(x_train, [75, 75])
    x_test = tf.image.resize(x_test, [75, 75])

# Input image dimensions.
input_shape = x_train.shape[1:]

if hvd.rank() == 0:
    print('x_train shape:', x_train.shape)
    print('y_train shape:', y_train.shape)
    print(x_train.shape[0], 'train samples')
    print(x_test.shape[0], 'test samples')

# Convert class vectors to binary class matrices.
y_train = tf.keras.utils.to_categorical(y_train, num_classes)
y_test = tf.keras.utils.to_categorical(y_test, num_classes)

train_gen = tf.keras.preprocessing.image.ImageDataGenerator(
        rotation_range=30,
        width_shift_range=0.1,
        height_shift_range=0.1,
        shear_range=0.,
        zoom_range=0.,
        fill_mode='nearest',
        horizontal_flip=True,
        vertical_flip=False,
        data_format=None,
        validation_split=0.0)
    
train_gen.fit(x_train)
train_iter = train_gen.flow(x_train, y_train, batch_size=batch_size)

test_gen = tf.keras.preprocessing.image.ImageDataGenerator(rotation_range=0)
test_gen.fit(x_test)
test_iter = test_gen.flow(x_test, y_test, batch_size=batch_size)

# Horovod: (optional) compression algorithm.
compression = hvd.Compression.fp16 if args.fp16_allreduce else hvd.Compression.none

# Set up standard model.
model = getattr(applications, model_name)(include_top=True,
                                          input_shape=input_shape,
                                          weights=None,
                                          classes=num_classes)
if hvd.rank() == 0:
    model.summary()

# Horovod: adjust learning rate based on number of GPUs.
scaled_lr = base_lr * hvd.size()

opt = getattr(tf.keras.optimizers, optimizer)(learning_rate=scaled_lr, momentum=momentum)

# Horovod: add Horovod DistributedOptimizer.
opt = hvd.DistributedOptimizer(opt,
                               backward_passes_per_step=1,
                               average_aggregated_gradients=True,
                               compression=compression)

model.compile(optimizer=opt,
              loss='categorical_crossentropy',
              metrics=['accuracy'],
              experimental_run_tf_function=False)

# Horovod: initialize optimizer state so we can synchronize across workers
# Keras has empty optimizer variables() for TF2:
# https://sourcegraph.com/github.com/tensorflow/tensorflow@v2.4.1/-/blob/tensorflow/python/keras/optimizer_v2/optimizer_v2.py#L351:10
model.fit(train_iter, steps_per_epoch=1, epochs=1, callbacks=None)

# Track epoch in the csv in the callback.
# Don't set append=False. Make sure empty csv when training from scratch,
# or epoch may be overrided in the callback.
metrics_logger = MetricsCSVLogger(metrics_path, append=True,
                                  total_epochs=epochs)
set_logger_params(metrics_logger, batch_size=batch_size, workers=hvd.size())

# Get init_epoch from csv
init_epoch = metrics_logger.epoch
if init_epoch > 0:
    model.load_weights(checkpoint_path)

state = hvd.elastic.KerasState(model, batch=0, epoch=init_epoch)

checkpoint = tf.keras.callbacks.ModelCheckpoint(checkpoint_path, save_weights_only=True)

callbacks = [
    hvd.elastic.UpdateEpochStateCallback(state),
    hvd.elastic.UpdateBatchStateCallback(state),
    hvd.elastic.CommitStateCallback(state, batches_per_commit=batches_per_commit),
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
    state.model.fit(train_iter, steps_per_epoch=1, epochs=1, callbacks=None)

# These callbacks are called after horovoe has reinitialized, but before state is synchronized across the workers.
state.register_reset_callbacks([on_state_reset])

# Train the model.
# Horovod: adjust number of steps based on number of GPUs.
@hvd.elastic.run
def train(state):
    state.model.fit(train_iter,
                    epochs=epochs-state.epoch,
                    steps_per_epoch=len(train_iter) // hvd.size(),
                    validation_data=test_iter,
                    validation_steps=len(test_iter) // hvd.size(),
                    callbacks=callbacks_root if hvd.rank() == 0 else callbacks,
                    verbose=0)

train(state)

# Evaluate the model.
if hvd.rank() == 0:
    scores = model.evaluate(x_test, y_test, verbose=2)
    print('Test loss:', scores[0])
    print('Test accuracy:', scores[1])