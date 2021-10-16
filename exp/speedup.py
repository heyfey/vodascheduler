import matplotlib.pyplot as plt


parallelism = [1, 2, 3, 4]

vgg = [1, 1.785, 2.5, 3.125]
resnet = [1, 1.857, 2.6, 3.25]
incv3 = [1, 1.842, 2.692, 3.5]
transformer = [1, 1.657, 2.148, 2.521]
ideal = [1, 2, 3, 4]

font_size = 24
linewidth = 4
markersize = 12
figure = plt.figure(figsize=(10, 8))
ax = figure.gca()
ax.xaxis.set_major_locator(MaxNLocator(integer=True))
# plt.title('title', fontsize=font_size)
plt.xlabel('Number of GPUs', fontsize=font_size)
plt.ylabel('Speedup', fontsize=font_size)


# plt.plot(parallelism, ideal, 'o-', color = 'k', label="linear speedup", linestyle='dashed')
# plt.plot(parallelism, incv3, 's-', color = 'r', label="InceptionV3")
# plt.plot(parallelism, resnet, 'v-', color = 'g', label="ResNet50")
# plt.plot(parallelism, vgg, '^-', color = 'b', label="VGG16")
# plt.plot(parallelism, transformer, 'D-', color = 'c', label="Transformer")

plt.plot(parallelism, ideal, 'o-', color = 'k', label="Linear speedup", linestyle='dashed', linewidth=linewidth, markersize=markersize)
# plt.plot(parallelism, ideal, 'o-', color = 'k', label="linear speedup", linestyle='dashed')
plt.plot(parallelism, incv3, 's-', label="InceptionV3", linewidth=linewidth, markersize=markersize)
plt.plot(parallelism, resnet, 'v-', label="ResNet50", linewidth=linewidth, markersize=markersize)
plt.plot(parallelism, vgg, '^-', label="VGG16", linewidth=linewidth, markersize=markersize)
plt.plot(parallelism, transformer, 'D-', label="Transformer", linewidth=linewidth, markersize=markersize)



plt.legend(fontsize=font_size)
plt.xticks(fontsize=font_size)
plt.yticks(fontsize=font_size)
plt.grid(color='k', linestyle='--', linewidth=1)
figure.tight_layout(pad=1.0)
plt.savefig('speedup.png')