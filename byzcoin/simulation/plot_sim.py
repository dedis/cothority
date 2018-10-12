import matplotlib.pyplot as plt
import pandas as pd
import os

data_dir = './test_data/'
files = [(data_dir + fname) for fname in os.listdir(data_dir)\
         if fname.startswith('coins') and fname.endswith('.csv')]

def read_all_files(files):
    df = pd.DataFrame()
    for fname in files:
        data = pd.read_csv(fname)
        # We need to add variables regarding
        # batching and keeping here.
        batch = '_batch' in fname
        keep = not '_nokeep' in fname
        rowcount = len(data.index)
        b_vals = pd.Series([batch for i in range(rowcount)])
        k_vals = pd.Series([keep for i in range(rowcount)])
        data = data.assign(batch=b_vals.values)
        data = data.assign(keep=k_vals.values)
        # If the data frame is empty (first iteration),
        # we append no matter what. Otherwise, we append
        # IFF the colums are the same.
        if df.empty \
           or (len(data.columns) == len(df.columns) \
           and (data.columns == df.columns).all()):
            df = df.append(data, ignore_index=True)
    return df

df = read_all_files(files)
delays = list(set(df['delay']))
keep = list(set(df['keep']))
batch = list(set(df['batch']))

for delay in delays:
    for k in keep:
        for b in batch:
            titlestring = 'Transactions: 1000, delay: {}, keep: {}, batch: {}'.format(delay, k, b)
            # No whitespace, colons or commata in filenames
            namestring = titlestring.replace(' ','').replace(':','-').replace(',','_')
            data = df.ix[df['delay'] == delay].sort_values('hosts')
            data = data.ix[data['keep'] == k]
            data = data.ix[data['batch'] == b]
            data = data.reset_index()

            ax = data.plot.bar(\
                    x='hosts',\
                    y=['prepare_wall_sum','send_wall_sum','confirm_wall_avg'],\
                    stacked=True)
            data.plot(y='round_wall_avg', marker='o', ax=ax)

            plt.xlabel('number of hosts')
            plt.ylabel('time in seconds')
            plt.title(titlestring)
            plt.savefig(data_dir + 'barplot_' + namestring + '.png')
            plt.close()


            ax = data.plot.bar(\
                    x='hosts',\
                    y=['prepare_wall_sum','send_wall_sum','confirm_wall_avg'],\
                    stacked=True)
            data.plot(y='round_wall_avg', marker='o', ax=ax)

            ax.set_yscale('log')

            plt.xlabel('number of hosts')
            plt.ylabel('logarithm of time in seconds')
            plt.title(titlestring)
            plt.savefig(data_dir + 'barplot_log_delay_' + namestring + '.png')
            plt.close()
