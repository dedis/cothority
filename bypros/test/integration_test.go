package bypros

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/bypros"
	"go.dedis.ch/cothority/v3/bypros/storage/sqlstore"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
)

// schemaRelativePath should point to a folder containing the schema of the
// database. This schema will be executed upon starting the container.
const schemaRelativePath = "../storage/sqlstore/schema"

const spawnRule = "spawn:value"
const countQuery = "select count(*) as result from cothority.transaction"

func TestIntegration_Simple(t *testing.T) {
	const url = "postgres://bypros:docker@localhost:4567/bypros"
	os.Setenv("PROXY_DB_URL", url)
	os.Setenv("PROXY_DB_URL_RO", url)

	cli, err := client.NewClientWithOpts()
	require.NoError(t, err)

	id := startPostgresDocker(t, cli, 4567)

	bct := byzcoin.NewBCTestDefault(t)

	// order is important: last-in first-out
	defer bct.CloseAll()
	defer cli.Close()
	defer stopPostgresDocker(t, cli, id)
	defer sqlstore.Registry.CloseAll()

	bct.AddGenesisRules(spawnRule)
	bct.CreateByzCoin()

	client := bypros.NewClient()

	// add 10 transactions
	for i := 0; i < 10; i++ {
		bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractValueID,
			},
		})
	}

	// start following
	err = client.Follow(bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash)
	require.NoError(t, err)

	// add again 10 transactions
	for i := 0; i < 10; i++ {
		bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractValueID,
			},
		})
	}

	// query the number of transactions, should be 10
	res, err := client.Query(bct.Roster.Get(0), countQuery)
	require.NoError(t, err)
	require.Equal(t, `[
  {
    "result": "10"
  }
]`, string(res))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// let's catch up
	resps, err := client.CatchUP(ctx, bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash, bct.Genesis.Hash, 5)
	require.NoError(t, err)

	expected := []bypros.CatchUpResponse{
		{
			Status: bypros.CatchUpStatus{
				Message:    "parsed block 4",
				BlockIndex: 4,
			},
		},
		{
			Status: bypros.CatchUpStatus{
				Message:    "parsed block 9",
				BlockIndex: 9,
			},
		},
		{
			Status: bypros.CatchUpStatus{
				Message:    "parsed block 14",
				BlockIndex: 14,
			},
		},
		{
			Status: bypros.CatchUpStatus{
				Message:    "parsed block 19",
				BlockIndex: 19,
			},
		},
		{
			Done: true,
		},
	}

	for i := 0; i < len(expected); i++ {
		select {
		case resp := <-resps:
			require.Equal(t, expected[i].Status.Message, resp.Status.Message)
			require.Equal(t, expected[i].Status.BlockIndex, resp.Status.BlockIndex)
			require.Equal(t, expected[i].Err, resp.Err)
			require.Equal(t, expected[i].Done, resp.Done)
		case <-time.After(time.Second):
			t.Error("didn't received after timeout")
		}
	}

	select {
	case resp, more := <-resps:
		require.False(t, more, resp)
	default:
	}

	// Now we should get 21 transactions: the 20 we added, + the genesis one to
	// create the config.
	res, err = client.Query(bct.Roster.Get(0), countQuery)
	require.NoError(t, err)
	require.Equal(t, `[
  {
    "result": "21"
  }
]`, string(res))

	err = client.Unfollow(bct.Roster.Get(0))
	require.NoError(t, err)
}

func TestIntegration_Intense(t *testing.T) {
	const url = "postgres://bypros:docker@localhost:4566/bypros"
	os.Setenv("PROXY_DB_URL", url)
	os.Setenv("PROXY_DB_URL_RO", url)

	n := 50

	// launch a routine to create N transactions
	// launch a routine to start following - stop following - start following
	// launch a routine to catchup
	// do a final catch up

	cli, err := client.NewClientWithOpts()
	require.NoError(t, err)

	id := startPostgresDocker(t, cli, 4566)
	defer stopPostgresDocker(t, cli, id)

	bct := byzcoin.NewBCTestDefault(t)

	// order is important: last-in first-out
	defer bct.CloseAll()
	defer cli.Close()
	defer sqlstore.Registry.CloseAll()

	bct.AddGenesisRules(spawnRule)
	bct.CreateByzCoin()

	client := bypros.NewClient()

	wait := sync.WaitGroup{}

	wait.Add(1)
	go func() {
		defer wait.Done()

		for i := 0; i < n; i++ {
			bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
				InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: contracts.ContractValueID,
				},
			})
		}
	}()

	time.Sleep(time.Second * 2)

	// start following
	err = client.Follow(bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash)
	require.NoError(t, err)

	wait.Add(1)
	go func() {
		defer wait.Done()

		time.Sleep(time.Second * 5)

		// Simulates a pause in following
		err = client.Unfollow(bct.Roster.Get(0))
		require.NoError(t, err)

		time.Sleep(time.Second * 5)

		// follow again
		err = client.Follow(bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash)
		require.NoError(t, err)
	}()

	wait.Add(1)
	go func() {
		defer wait.Done()

		// let the system already create some blocks
		time.Sleep(time.Second * 10)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// let's catch up
		resps, err := client.CatchUP(ctx, bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash, bct.Genesis.Hash, 100)
		require.NoError(t, err)

		for resp := range resps {
			t.Log(resp)
		}
	}()

	wait.Wait()

	// let's do a last catch up, in case the previous one was faster than the
	// addition of transactions and stopped.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// let's catch up
	resps, err := client.CatchUP(ctx, bct.Roster.Get(0), bct.Roster.Get(0), bct.Genesis.Hash, bct.Genesis.Hash, 100)
	require.NoError(t, err)

	for resp := range resps {
		t.Log(resp)
	}

	err = client.Unfollow(bct.Roster.Get(0))
	require.NoError(t, err)

	// Now we should get 21 transactions: the 20 we added, + the genesis one to
	// create the config.
	res, err := client.Query(bct.Roster.Get(0), countQuery)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(`[
  {
    "result": "%d"
  }
]`, n+1), string(res))
}

// -----------------------------------------------------------------------------
// Utility functions

func startPostgresDocker(t *testing.T, cli *client.Client, port int) string {
	ctx := context.Background()

	res, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        "postgres",
		ExposedPorts: nat.PortSet{"5432": struct{}{}},
		Env:          []string{"POSTGRES_PASSWORD=docker", "POSTGRES_USER=bypros", "POSTGRES_DB=bypros"},
	}, &container.HostConfig{
		PortBindings: map[nat.Port][]nat.PortBinding{nat.Port("5432"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", port)}}},
	}, &network.NetworkingConfig{}, nil, "")

	require.NoError(t, err)

	archive, err := NewTarArchiveFromPath(schemaRelativePath)
	require.NoError(t, err)

	err = cli.CopyToContainer(ctx, res.ID, "/docker-entrypoint-initdb.d", archive, types.CopyToContainerOptions{})
	require.NoError(t, err)

	err = cli.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	t.Log("docker started")

	return res.ID
}

func stopPostgresDocker(t *testing.T, cli *client.Client, id string) {
	ctx := context.Background()

	err := cli.ContainerStop(ctx, id, nil)
	require.NoError(t, err)
	t.Log("docker stopped")

	err = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{RemoveVolumes: true})
	require.NoError(t, err)
	t.Log("docker removed")
}

func NewTarArchiveFromPath(path string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	ok := filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(strings.Replace(file, path, "", -1), string(filepath.Separator))
		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}
		return nil
	})

	if ok != nil {
		return nil, ok
	}
	ok = tw.Close()
	if ok != nil {
		return nil, ok
	}
	return bufio.NewReader(&buf), nil
}
