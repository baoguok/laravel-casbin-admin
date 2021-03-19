package roadrunner

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/spiral/errors"
	"github.com/stretchr/testify/assert"
)

func Test_Echo(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, err := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := syncWorker.Exec(Payload{Body: []byte("hello")})

	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Equal(t, "hello", res.String())
}

func Test_BadPayload(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := syncWorker.Exec(EmptyPayload)

	assert.Error(t, err)
	assert.Nil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Contains(t, err.Error(), "payload can not be empty")
}

func Test_NotStarted_String(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := InitBaseWorker(cmd)
	assert.Contains(t, w.String(), "php tests/client.php echo pipes")
	assert.Contains(t, w.String(), "inactive")
	assert.Contains(t, w.String(), "numExecs: 0")
}

func Test_NotStarted_Exec(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := InitBaseWorker(cmd)

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := syncWorker.Exec(Payload{Body: []byte("hello")})

	assert.Error(t, err)
	assert.Nil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Contains(t, err.Error(), "WorkerProcess is not ready (inactive)")
}

func Test_String(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	assert.Contains(t, w.String(), "php tests/client.php echo pipes")
	assert.Contains(t, w.String(), "ready")
	assert.Contains(t, w.String(), "numExecs: 0")
}

func Test_Echo_Slow(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/slow-client.php", "echo", "pipes", "10", "10")

	w, _ := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := syncWorker.Exec(Payload{Body: []byte("hello")})

	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Equal(t, "hello", res.String())
}

func Test_Broken(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "broken", "pipes")

	w, err := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan struct{})

	go func() {
		tt := time.NewTimer(time.Second * 10)
		select {
		case <-tt.C:
			tt.Stop()
			ch <- struct{}{}
		}
	}()

	w.AddListener(func(event interface{}) {
		assert.Contains(t, string(event.(WorkerEvent).Payload.([]byte)), "undefined_function()")
		ch <- struct{}{}
	})

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := syncWorker.Exec(Payload{Body: []byte("hello")})
	assert.NotNil(t, err)
	assert.Nil(t, res.Body)
	assert.Nil(t, res.Context)

	<-ch
	assert.Error(t, w.Stop(ctx))
}

func Test_Error(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "error", "pipes")

	w, _ := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()

	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := syncWorker.Exec(Payload{Body: []byte("hello")})
	assert.NotNil(t, err)
	assert.Nil(t, res.Body)
	assert.Nil(t, res.Context)

	if errors.Is(errors.ErrSoftJob, err) == false {
		t.Fatal("error should be of type errors.ErrSoftJob")
	}
	assert.Contains(t, err.Error(), "exec payload: SoftJobError: hello")
}

func Test_NumExecs(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorkerWithContext(ctx, cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	syncWorker, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	_, err = syncWorker.Exec(Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(1), w.State().NumExecs())

	_, err = syncWorker.Exec(Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(2), w.State().NumExecs())

	_, err = syncWorker.Exec(Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(3), w.State().NumExecs())
}
