package roadrunner

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var cfgSupervised = PoolConfig{
	NumWorkers:      int64(1),
	AllocateTimeout: time.Second,
	DestroyTimeout:  time.Second,
	Supervisor: &SupervisorConfig{
		WatchTick:       1,
		TTL:             100,
		IdleTTL:         100,
		ExecTTL:         100,
		MaxWorkerMemory: 100,
	},
}

func TestSupervisedPool_Exec(t *testing.T) {
	ctx := context.Background()
	p, err := NewPool(
		ctx,
		func() *exec.Cmd { return exec.Command("php", "tests/memleak.php", "pipes") },
		NewPipeFactory(),
		cfgSupervised,
	)

	assert.NoError(t, err)
	assert.NotNil(t, p)
	stopCh := make(chan struct{})
	defer p.Destroy(context.Background())

	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				workers := p.Workers()
				if len(workers) > 0 {
					s, err := WorkerProcessState(workers[0])
					assert.NoError(t, err)
					assert.NotNil(t, s)
					// since this is soft limit, double max memory limit watch
					if (s.MemoryUsage / MB) > cfgSupervised.Supervisor.MaxWorkerMemory*2 {
						assert.Fail(t, "max memory reached")
					}
				}
			}
		}
	}()

	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond * 50)
		_, err = p.Exec(Payload{
			Context: []byte(""),
			Body:    []byte("foo"),
		})
		assert.NoError(t, err)
	}

	stopCh <- struct{}{}
}

func TestSupervisedPool_ExecTTL_TimedOut(t *testing.T) {
	var cfgExecTTL = PoolConfig{
		NumWorkers:      int64(1),
		AllocateTimeout: time.Second,
		DestroyTimeout:  time.Second,
		Supervisor: &SupervisorConfig{
			WatchTick:       1,
			TTL:             100,
			IdleTTL:         100,
			ExecTTL:         1,
			MaxWorkerMemory: 100,
		},
	}
	ctx := context.Background()
	p, err := NewPool(
		ctx,
		func() *exec.Cmd { return exec.Command("php", "tests/sleep.php", "pipes") },
		NewPipeFactory(),
		cfgExecTTL,
	)

	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Destroy(context.Background())

	pid := p.Workers()[0].Pid()

	resp, err := p.ExecWithContext(context.Background(), Payload{
		Context: []byte(""),
		Body:    []byte("foo"),
	})

	assert.Error(t, err)
	assert.Empty(t, resp)

	time.Sleep(time.Second * 1)
	// should be new worker with new pid
	assert.NotEqual(t, pid, p.Workers()[0].Pid())
}

func TestSupervisedPool_ExecTTL_OK(t *testing.T) {
	var cfgExecTTL = PoolConfig{
		NumWorkers:      int64(1),
		AllocateTimeout: time.Second,
		DestroyTimeout:  time.Second,
		Supervisor: &SupervisorConfig{
			WatchTick:       1,
			TTL:             100,
			IdleTTL:         100,
			ExecTTL:         4,
			MaxWorkerMemory: 100,
		},
	}
	ctx := context.Background()
	p, err := NewPool(
		ctx,
		func() *exec.Cmd { return exec.Command("php", "tests/sleep.php", "pipes") },
		NewPipeFactory(),
		cfgExecTTL,
	)

	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Destroy(context.Background())

	pid := p.Workers()[0].Pid()

	time.Sleep(time.Millisecond * 100)
	resp, err := p.Exec(Payload{
		Context: []byte(""),
		Body:    []byte("foo"),
	})

	assert.NoError(t, err)
	assert.Empty(t, resp)

	time.Sleep(time.Second * 1)
	// should be the same pid
	assert.Equal(t, pid, p.Workers()[0].Pid())
}
