import os
import time

cwd = os.getcwd()
cmd_path = os.path.join(os.path.dirname(cwd), 'cmd')
os.chdir(cmd_path)

mnist_short = "../test/tensorflow2-keras-mnist-elastic-short.yaml"
mnist_medium = "../test/tensorflow2-keras-mnist-elastic-medium.yaml"
mnist_long = "../test/tensorflow2-keras-mnist-elastic-long.yaml"

os.system("go run main.go create -f {}".format(mnist_medium))
time.sleep(30)
os.system("go run main.go create -f {}".format(mnist_medium))
time.sleep(30)
os.system("go run main.go create -f {}".format(mnist_medium))
time.sleep(30)
os.system("go run main.go create -f {}".format(mnist_medium))
time.sleep(30)
os.system("go run main.go create -f {}".format(mnist_medium))
# time.sleep(30)
# os.system("go run main.go create -f {}".format(mnist_medium))
# time.sleep(30)
# os.system("go run main.go create -f {}".format(mnist_medium))
# time.sleep(30)
# os.system("go run main.go create -f {}".format(mnist_medium))
# time.sleep(30)
# os.system("go run main.go create -f {}".format(mnist_medium))
# time.sleep(30)
# os.system("go run main.go create -f {}".format(mnist_medium))


# os.system("go run main.go delete xxx")
# os.system("go run main.go get jobs")