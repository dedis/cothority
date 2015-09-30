import matplotlib.pyplot as plt
import csv

with open('sign_multi.csv') as csvfile:
    reader = csv.DictReader(csvfile)
    for row in reader:
        print(row['hosts'])


plt.plot([1,2,3,4])
plt.ylabel('some numbers')
plt.show()
