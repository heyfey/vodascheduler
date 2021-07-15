"""
Title: English-to-Spanish translation with a sequence-to-sequence Transformer
Author: [fchollet](https://twitter.com/fchollet)
Date created: 2021/05/26
Last modified: 2021/05/26
Description: Implementing a sequence-to-sequene Transformer and training it on a machine translation task.
"""

import pathlib
import random
import string
import re
import numpy as np
import tensorflow as tf
from tensorflow import keras
from tensorflow.keras import layers
from keras.layers.experimental.preprocessing import TextVectorization
import layers_tf25

import os
import horovod.tensorflow.keras as hvd
from callbacks import MetricsCSVLogger, set_logger_params
from argparse import ArgumentParser


parser = ArgumentParser(description='Celeste tf.keras Transformer Example')
parser.add_argument("--name"              , dest ="job_name", type=str, default='', help="unique name to identify training job (default: file name)")
parser.add_argument("--data-dir"          , dest ="d_dir", type=str, default='./', help="path to dataset")
parser.add_argument("--store-dir"         , dest ="s_dir", type=str, default='./', help="path to outputs (checkpoint)")
parser.add_argument("--metrics-dir"       , dest ="m_dir", type=str, default='./', help="path to metrics")
parser.add_argument('--checkpoint-format' , dest ="checkpoint_format", type=str, default='checkpoint.h5', help='checkpoint file format')

parser.add_argument("--lr"                , dest ="lr", type=float, default=0.001, help="base learning rate for a single GPU")
parser.add_argument("--batch-size"        , dest ="batch_size", type=int, default=512, help="local batch size")
parser.add_argument("--epochs"            , dest ="epochs", type=int, default=30, help="number of epochs to train")
parser.add_argument('--fp16-allreduce'    , dest ="fp16_allreduce", action='store_true', default=False, help='use fp16 compression during allreduce')
parser.add_argument('--warmup-epochs'     , dest ="warmup_epochs", type=int, default=0, help='number of warmup epochs')
parser.add_argument('--batches-per-commit', dest ="batches_per_commit", type=int, default=100, help='commit state every # batches')

args = parser.parse_args()

# set and print hyper-parameters
job_name           = args.job_name
dataset_dir        = args.d_dir
store_dir          = args.s_dir
metrics_dir        = args.m_dir
checkpoint_format  = args.checkpoint_format
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

# For dataset
vocab_size = 15000
sequence_length = 20
# For model
embed_dim = 256
latent_dim = 2048
num_heads = 8

## Downloading the data
# We'll be working with an English-to-Spanish translation dataset
# provided by [Anki](https://www.manythings.org/anki/). Let's download it:
text_file = keras.utils.get_file(
    fname="spa-eng.zip",
    origin="http://storage.googleapis.com/download.tensorflow.org/data/spa-eng.zip",
    extract=True,
)
text_file = pathlib.Path(text_file).parent / "spa-eng" / "spa.txt"

## Parsing the data
with open(text_file) as f:
    lines = f.read().split("\n")[:-1]
text_pairs = []
for line in lines:
    eng, spa = line.split("\t")
    spa = "[start] " + spa + " [end]"
    text_pairs.append((eng, spa))


## Split the sentence pairs into a training set, a validation set, and a test set
random.shuffle(text_pairs)
num_val_samples = int(0.15 * len(text_pairs))
num_train_samples = len(text_pairs) - 2 * num_val_samples
train_pairs = text_pairs[:num_train_samples]
val_pairs = text_pairs[num_train_samples : num_train_samples + num_val_samples]
test_pairs = text_pairs[num_train_samples + num_val_samples :]

print(f"{len(text_pairs)} total pairs")
print(f"{len(train_pairs)} training pairs")
print(f"{len(val_pairs)} validation pairs")
print(f"{len(test_pairs)} test pairs")

# Calulate steps_per_epoch
steps_per_epoch = int(len(train_pairs) // batch_size)
validation_steps = int(len(val_pairs) // batch_size)
print("steps_per_epoch: {}".format(steps_per_epoch))
print("validation_steps: {}".format(validation_steps))

## Vectorizing the text data
strip_chars = string.punctuation + "¿"
strip_chars = strip_chars.replace("[", "")
strip_chars = strip_chars.replace("]", "")


def custom_standardization(input_string):
    lowercase = tf.strings.lower(input_string)
    return tf.strings.regex_replace(lowercase, "[%s]" % re.escape(strip_chars), "")


eng_vectorization = TextVectorization(
    max_tokens=vocab_size, output_mode="int", output_sequence_length=sequence_length,
)
spa_vectorization = TextVectorization(
    max_tokens=vocab_size,
    output_mode="int",
    output_sequence_length=sequence_length + 1,
    standardize=custom_standardization,
)
train_eng_texts = [pair[0] for pair in train_pairs]
train_spa_texts = [pair[1] for pair in train_pairs]
eng_vectorization.adapt(train_eng_texts)
spa_vectorization.adapt(train_spa_texts)


## Format the datasets.
def format_dataset(eng, spa):
    eng = eng_vectorization(eng)
    spa = spa_vectorization(spa)
    return ({"encoder_inputs": eng, "decoder_inputs": spa[:, :-1],}, spa[:, 1:])


def make_dataset(pairs):
    eng_texts, spa_texts = zip(*pairs)
    eng_texts = list(eng_texts)
    spa_texts = list(spa_texts)
    dataset = tf.data.Dataset.from_tensor_slices((eng_texts, spa_texts))
    dataset = dataset.cache()
    dataset = dataset.batch(batch_size)
    dataset = dataset.map(format_dataset)
    return dataset.shuffle(2048).repeat().prefetch(16)


train_ds = make_dataset(train_pairs)
val_ds = make_dataset(val_pairs)


## Let's take a quick look at the sequence shapes
for inputs, targets in train_ds.take(1):
    print(f'inputs["encoder_inputs"].shape: {inputs["encoder_inputs"].shape}')
    print(f'inputs["decoder_inputs"].shape: {inputs["decoder_inputs"].shape}')
    print(f"targets.shape: {targets.shape}")


## Building the model
class TransformerEncoder(layers.Layer):
    def __init__(self, embed_dim, dense_dim, num_heads, **kwargs):
        super(TransformerEncoder, self).__init__(**kwargs)
        self.embed_dim = embed_dim
        self.dense_dim = dense_dim
        self.num_heads = num_heads
        self.attention = layers_tf25.MultiHeadAttention(
            num_heads=num_heads, key_dim=embed_dim
        )
        self.dense_proj = keras.Sequential(
            [layers.Dense(dense_dim, activation="relu"), layers.Dense(embed_dim),]
        )
        self.layernorm_1 = layers.LayerNormalization()
        self.layernorm_2 = layers.LayerNormalization()
        self.supports_masking = True

    def call(self, inputs, mask=None):
        if mask is not None:
            padding_mask = tf.cast(mask[:, tf.newaxis, tf.newaxis, :], dtype="int32")
        attention_output = self.attention(
            query=inputs, value=inputs, key=inputs, attention_mask=padding_mask
        )
        proj_input = self.layernorm_1(inputs + attention_output)
        proj_output = self.dense_proj(proj_input)
        return self.layernorm_2(proj_input + proj_output)


class PositionalEmbedding(layers.Layer):
    def __init__(self, sequence_length, vocab_size, embed_dim, **kwargs):
        super(PositionalEmbedding, self).__init__(**kwargs)
        self.token_embeddings = layers.Embedding(
            input_dim=vocab_size, output_dim=embed_dim
        )
        self.position_embeddings = layers.Embedding(
            input_dim=sequence_length, output_dim=embed_dim
        )
        self.sequence_length = sequence_length
        self.vocab_size = vocab_size
        self.embed_dim = embed_dim

    def call(self, inputs):
        length = tf.shape(inputs)[-1]
        positions = tf.range(start=0, limit=length, delta=1)
        embedded_tokens = self.token_embeddings(inputs)
        embedded_positions = self.position_embeddings(positions)
        return embedded_tokens + embedded_positions

    def compute_mask(self, inputs, mask=None):
        return tf.math.not_equal(inputs, 0)


class TransformerDecoder(layers.Layer):
    def __init__(self, embed_dim, latent_dim, num_heads, **kwargs):
        super(TransformerDecoder, self).__init__(**kwargs)
        self.embed_dim = embed_dim
        self.latent_dim = latent_dim
        self.num_heads = num_heads
        self.attention_1 = layers_tf25.MultiHeadAttention(
            num_heads=num_heads, key_dim=embed_dim
        )
        self.attention_2 = layers_tf25.MultiHeadAttention(
            num_heads=num_heads, key_dim=embed_dim
        )
        self.dense_proj = keras.Sequential(
            [layers.Dense(latent_dim, activation="relu"), layers.Dense(embed_dim),]
        )
        self.layernorm_1 = layers.LayerNormalization()
        self.layernorm_2 = layers.LayerNormalization()
        self.layernorm_3 = layers.LayerNormalization()
        self.supports_masking = True

    def call(self, inputs, encoder_outputs, mask=None):
        causal_mask = self.get_causal_attention_mask(inputs)
        if mask is not None:
            padding_mask = tf.cast(mask[:, tf.newaxis, :], dtype="int32")
            padding_mask = tf.minimum(padding_mask, causal_mask)

        attention_output_1 = self.attention_1(
            query=inputs, value=inputs, key=inputs, attention_mask=causal_mask
        )
        out_1 = self.layernorm_1(inputs + attention_output_1)

        attention_output_2 = self.attention_2(
            query=out_1,
            value=encoder_outputs,
            key=encoder_outputs,
            attention_mask=padding_mask,
        )
        out_2 = self.layernorm_2(out_1 + attention_output_2)

        proj_output = self.dense_proj(out_2)
        return self.layernorm_3(out_2 + proj_output)

    def get_causal_attention_mask(self, inputs):
        input_shape = tf.shape(inputs)
        batch_size, sequence_length = input_shape[0], input_shape[1]
        i = tf.range(sequence_length)[:, tf.newaxis]
        j = tf.range(sequence_length)
        mask = tf.cast(i >= j, dtype="int32")
        mask = tf.reshape(mask, (1, input_shape[1], input_shape[1]))
        mult = tf.concat(
            [tf.expand_dims(batch_size, -1), tf.constant([1, 1], dtype=tf.int32)],
            axis=0,
        )
        return tf.tile(mask, mult)


## Assemble the end-to-end model.
encoder_inputs = keras.Input(shape=(None,), dtype="int64", name="encoder_inputs")
x = PositionalEmbedding(sequence_length, vocab_size, embed_dim)(encoder_inputs)
encoder_outputs = TransformerEncoder(embed_dim, latent_dim, num_heads)(x)
encoder = keras.Model(encoder_inputs, encoder_outputs)

decoder_inputs = keras.Input(shape=(None,), dtype="int64", name="decoder_inputs")
encoded_seq_inputs = keras.Input(shape=(None, embed_dim), name="decoder_state_inputs")
x = PositionalEmbedding(sequence_length, vocab_size, embed_dim)(decoder_inputs)
x = TransformerDecoder(embed_dim, latent_dim, num_heads)(x, encoded_seq_inputs)
x = layers.Dropout(0.5)(x)
decoder_outputs = layers.Dense(vocab_size, activation="softmax")(x)
decoder = keras.Model([decoder_inputs, encoded_seq_inputs], decoder_outputs)

decoder_outputs = decoder([decoder_inputs, encoder_outputs])
transformer = keras.Model(
    [encoder_inputs, decoder_inputs], decoder_outputs, name="transformer"
)
transformer.summary()

# Horovod: (optional) compression algorithm.
compression = hvd.Compression.fp16 if fp16_allreduce else hvd.Compression.none

# Horovod: adjust learning rate based on number of GPUs.
scaled_lr = base_lr * hvd.size()

opt = tf.optimizers.RMSprop(scaled_lr)

# Horovod: add Horovod DistributedOptimizer.
opt = hvd.DistributedOptimizer(opt,
                               backward_passes_per_step=1,
                               average_aggregated_gradients=True,
                               compression=compression)

## Training the model
# Horovod: Specify `experimental_run_tf_function=False` to ensure TensorFlow
# uses hvd.DistributedOptimizer() to compute gradients.
transformer.compile(
    optimizer=opt, loss="sparse_categorical_crossentropy", metrics=["accuracy"],
    experimental_run_tf_function=False
)

# Horovod: initialize optimizer state so we can synchronize across workers
# Keras has empty optimizer variables() for TF2:
# https://sourcegraph.com/github.com/tensorflow/tensorflow@v2.4.1/-/blob/tensorflow/python/keras/optimizer_v2/optimizer_v2.py#L351:10
transformer.fit(train_ds, steps_per_epoch=1, epochs=1, callbacks=None)

# Track epoch in the csv in the callback.
# Don't set append=False. Make sure empty csv when training from scratch,
# or epoch may be overrided in the callback.
metrics_logger = MetricsCSVLogger(metrics_path, append=True,
                                  total_epochs=epochs)
set_logger_params(metrics_logger, batch_size=batch_size, workers=hvd.size())

# Get init_epoch from csv
init_epoch = metrics_logger.epoch
if init_epoch > 0:
    transformer.load_weights(checkpoint_path)

state = hvd.elastic.KerasState(transformer, batch=0, epoch=init_epoch)

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
    state.model.fit(train_ds, steps_per_epoch=1, epochs=1, callbacks=None)

# These callbacks are called after horovoe has reinitialized, but before state is synchronized across the workers.
state.register_reset_callbacks([on_state_reset])

# Train the model.
# Horovod: adjust number of steps based on number of GPUs.
@hvd.elastic.run
def train(state):
    state.model.fit(train_ds,
                    epochs=epochs,
                    steps_per_epoch=steps_per_epoch // hvd.size(),
                    validation_data=val_ds,
                    validation_steps=validation_steps,
                    callbacks=callbacks_root if hvd.rank() == 0 else callbacks,
                    verbose=0)

train(state)

if hvd.rank() == 0:
    ## Decoding test sentences
    spa_vocab = spa_vectorization.get_vocabulary()
    spa_index_lookup = dict(zip(range(len(spa_vocab)), spa_vocab))
    max_decoded_sentence_length = 20

    def decode_sequence(input_sentence):
        tokenized_input_sentence = eng_vectorization([input_sentence])
        decoded_sentence = "[start]"
        for i in range(max_decoded_sentence_length):
            tokenized_target_sentence = spa_vectorization([decoded_sentence])[:, :-1]
            predictions = transformer([tokenized_input_sentence, tokenized_target_sentence])

            sampled_token_index = np.argmax(predictions[0, i, :])
            sampled_token = spa_index_lookup[sampled_token_index]
            decoded_sentence += " " + sampled_token

            if sampled_token == "[end]":
                break
        return decoded_sentence


    test_eng_texts = [pair[0] for pair in test_pairs]
    for _ in range(30):
        input_sentence = random.choice(test_eng_texts)
        translated = decode_sequence(input_sentence)
        print("input: {}".format(input_sentence))
        print("output: {}".format(translated))
