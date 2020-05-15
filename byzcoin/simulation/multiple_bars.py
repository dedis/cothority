import matplotlib.pyplot as plt
from matplotlib.dates import date2num
import datetime

x = [
    datetime.datetime(2011, 1, 4, 0, 0),
    datetime.datetime(2011, 1, 5, 0, 0),
    datetime.datetime(2011, 1, 6, 0, 0)
]
x = date2num(x)

y = [4, 9, 2]
z = [1, 2, 3]
k = [11, 12, 13]

ax = plt.subplot(111)
ax.bar(x-0.2, y, width=0.2, color='b', align='center')
ax.bar(x, z, width=0.2, color='g', align='center')
ax.bar(x+0.2, k, width=0.2, color='r', align='center')



labels = ["20/1", "20/5", "20/10", "200/1", "200/5", "200/10", "2000/1", "2000/5", "2000/10", "2000/100"]
ax.set_xticklabels(labels)
plt.xlabel('transactions / instructions')
plt.ylabel('Time in seconds')

plt.show()