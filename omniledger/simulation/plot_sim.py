import matplotlib.pyplot as plt
import pandas as pd
import os, sys

data_dir = './test_data/'


files = [(data_dir + fname) for fname in os.listdir(data_dir)\
         if fname.startswith('service') and fname.endswith('.csv')]

def read_all_files(files):
    df = pd.DataFrame()
    for file in files:
        data = pd.read_csv(file)
        if df.empty \
           or len(data.columns) == len(df.columns) \
           and (data.columns == df.columns).all():
            df = df.append(data, ignore_index=True)
    return df

df = read_all_files(files)

delays = list(set(df['delay']))

for delay in delays:
    data = df.ix[df['delay'] == delay].sort_values('hosts')

    data.plot.bar(x='hosts',\
                  y=['prepare_wall_avg','send_wall_avg','confirm_wall_avg'],\
                  stacked=True)
    plt.xlabel('number of hosts')
    plt.ylabel('time in seconds')
    plt.savefig(data_dir + 'barplot_delay_' + str(delay) + '.png')

    data.plot.bar(x='hosts',\
                  y=['prepare_wall_avg','send_wall_avg'],\
                  stacked=True)
    plt.xlabel('number of hosts')
    plt.ylabel('time in seconds')
    plt.savefig(data_dir + 'barplot_delay_' + str(delay) + '_noconfirm.png')

    data.plot.bar(x='hosts',\
                  y=['prepare_wall_avg','send_wall_avg','confirm_wall_avg'],\
                  stacked=True, log=True)
    plt.xlabel('number of hosts')
    plt.ylabel('logarithm of time in seconds')
    plt.savefig(data_dir + 'barplot_log_delay_' + str(delay) + '_noconfirm.png')


