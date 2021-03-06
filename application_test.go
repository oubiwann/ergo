package ergo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halturin/ergo/etf"
)

type testApplication struct {
	Application
}

func (a *testApplication) Load(args ...interface{}) (ApplicationSpec, error) {
	lifeSpan := args[0].(time.Duration)
	name := args[1].(string)
	nameGS := "testAppGS"
	if len(args) == 3 {
		nameGS = args[2].(string)
	}
	return ApplicationSpec{
		Name:        name,
		Description: "My Test Applicatoin",
		Version:     "v.0.1",
		Environment: map[string]interface{}{
			"envName1": 123,
			"envName2": "Hello world",
		},
		Children: []ApplicationChildSpec{
			ApplicationChildSpec{
				Child: &testAppGenServer{},
				Name:  nameGS,
			},
		},
		Lifespan: lifeSpan,
	}, nil
}

func (a *testApplication) Start(p *Process, args ...interface{}) {
	//p.SetEnv("env123", 456)
}

// test GenServer
type testAppGenServer struct {
	GenServer
}

func (gs *testAppGenServer) Init(p *Process, args ...interface{}) interface{} {
	//fmt.Println("STARTING TEST GS IN APP")
	p.SetEnv("env123", 456)
	return nil
}

func (gs *testAppGenServer) HandleCast(message etf.Term, state interface{}) (string, interface{}) {
	return "noreply", state
}

func (gs *testAppGenServer) HandleCall(from etf.Tuple, message etf.Term, state interface{}) (string, etf.Term, interface{}) {
	return "stop", message, nil
}

func (gs *testAppGenServer) HandleInfo(message etf.Term, state interface{}) (string, interface{}) {
	return "noreply", state
}

func (gs *testAppGenServer) Terminate(reason string, state interface{}) {
	//fmt.Println("TERMINATING TEST GS IN APP with reason:", reason)
}

// testing application
func TestApplication(t *testing.T) {

	fmt.Printf("\n=== Test Application load/unload/start/stop\n")
	fmt.Printf("\nStarting node nodeTestAplication@localhost:")
	ctx := context.Background()
	node := CreateNodeWithContext(ctx, "nodeTestApplication@localhost", "cookies", NodeOptions{})
	if node == nil {
		t.Fatal("can't start node")
	} else {
		fmt.Println("OK")
	}

	app := &testApplication{}
	lifeSpan := 0 * time.Second

	//
	// case 1: loading/unloading app
	//
	fmt.Printf("Loading application... ")
	err := node.ApplicationLoad(app, lifeSpan, "testapp")
	if err != nil {
		t.Fatal(err)
	}

	la := node.LoadedApplications()

	if len(la) != 1 {
		t.Fatal("total number of loaded application mismatch")
	}
	if la[0].Name != "testapp" {
		t.Fatal("can't load application")
	}

	fmt.Println("OK")

	wa := node.WhichApplications()
	if len(wa) > 0 {
		t.Fatal("total number of running application mismatch")
	}

	fmt.Printf("Unloading application... ")
	if err := node.ApplicationUnload("testapp"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // takes some time
	la = node.LoadedApplications()

	if len(la) > 0 {
		t.Fatal("total number of loaded application mismatch")
	}
	fmt.Println("OK")

	//
	// case 2: start(and try to unload running app)/stop(normal) application
	//
	fmt.Printf("Starting application... ")
	// use the new app name because the unloading takes some time
	if err := node.ApplicationLoad(app, lifeSpan, "testapp1", "testAppGS1"); err != nil {
		t.Fatal(err)
	}

	p, e := node.ApplicationStart("testapp1")
	if e != nil {
		t.Fatal(e)
	}

	// we shouldn't be able to unload running app
	if e := node.ApplicationUnload("testapp1"); e != ErrAppAlreadyStarted {
		t.Fatal(e)
	}

	wa = node.WhichApplications()
	if len(wa) != 1 {
		t.Fatal("total number of running application mismatch")
	}

	if wa[0].Name != "testapp1" {
		t.Fatal("can't start application")
	}

	fmt.Println("OK")

	// case 2.1: test env vars
	fmt.Printf("testing application' environment variables...")
	p.SetEnv("env123", 123)
	p.SetEnv("envStr", "123")

	gs := node.GetProcessByName("testAppGS1")
	env := gs.GetEnv("env123")
	if env == nil {
		t.Fatal("incorrect environment variable: not found")
	}

	if env.(int) != 456 {
		t.Fatal("incorrect environment variable: value should be overrided by child process")
	}

	if envUnknown := gs.GetEnv("unknown"); envUnknown != nil {
		t.Fatal("incorrect environment variable: undefined variable should have nil value")
	}

	envs := gs.ListEnv()
	if x, ok := envs["env123"]; !ok || x != 456 {
		t.Fatal("incorrect environment variable: list of variables has no env123 value or its wrong")
	}

	if x, ok := envs["envStr"]; !ok || x != "123" {
		t.Fatal("incorrect environment variable: list of variables has no envStr value or its wrong")
	}

	fmt.Println("OK")

	// case 2.2: get list of children' pid
	fmt.Printf("testing application' children list...")
	list := p.GetChildren()
	if len(list) != 1 || list[0] != gs.Self() {
		t.Fatal("incorrect children list")
	}
	fmt.Println("OK")

	// case 2.3: get application info

	fmt.Printf("get application info for testapp1...")
	info, errInfo := node.GetApplicationInfo("testapp1")
	if errInfo != nil {
		t.Fatal(errInfo)
	}
	if p.Self() != info.PID {
		t.Fatal("incorrect pid in application info")
	}
	fmt.Println("OK")

	fmt.Printf("Stopping application ...")
	if e := node.ApplicationStop("testapp1"); e != nil {
		t.Fatal(e)
	}
	fmt.Println("OK")

	if e := p.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("timed out")
	}
	wa = node.WhichApplications()
	if len(wa) != 0 {
		fmt.Println("waa: ", wa)
		t.Fatal("total number of running application mismatch")
	}

	//
	// case 3: start/stop (brutal) application
	//
	fmt.Printf("Starting application for brutal kill...")
	p, e = node.ApplicationStart("testapp1")
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("OK")
	fmt.Printf("Kill application...")
	p.Kill()
	if e := p.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("timed out")
	}
	fmt.Println("OK")

	node.ApplicationUnload("testapp1")

	//
	// case 4: start with limited lifespan
	//
	fmt.Printf("Starting application with lifespan 150ms...")
	lifeSpan = 150 * time.Millisecond
	if err := node.ApplicationLoad(app, lifeSpan, "testapp2", "testAppGS2"); err != nil {
		t.Fatal(err)
	}
	tStart := time.Now()
	p, e = node.ApplicationStart("testapp2")
	if e != nil {
		t.Fatal(e)
	}
	if e := p.WaitWithTimeout(160 * time.Millisecond); e != nil {
		t.Fatal("application lifespan was longer than 150m")
	}
	tLifeSpan := time.Since(tStart)

	if node.IsProcessAlive(p.Self()) {
		t.Fatal("application still alive")
	}

	if tLifeSpan < lifeSpan {
		t.Fatal("application lifespan was shorter(", tLifeSpan, ") than ", lifeSpan)
	}

	fmt.Println("OK. lifespan:", tLifeSpan)

	node.Stop()
}

func TestApplicationTypePermanent(t *testing.T) {
	fmt.Printf("\n=== Test Application type Permanent\n")
	fmt.Printf("\nStarting node nodeTestAplicationPermanent@localhost:")
	ctx := context.Background()
	node := CreateNodeWithContext(ctx, "nodeTestApplicationPermanent@localhost", "cookies", NodeOptions{})
	if node == nil {
		t.Fatal("can't start node")
	} else {
		fmt.Println("OK")
	}

	fmt.Printf("Starting application... ")
	app := &testApplication{}
	lifeSpan := time.Duration(0)
	if err := node.ApplicationLoad(app, lifeSpan, "testapp"); err != nil {
		t.Fatal(err)
	}

	p, e := node.ApplicationStartPermanent("testapp")
	if e != nil {
		t.Fatal(e)
	}

	gs := node.GetProcessByName("testAppGS")

	gs.Exit(p.Self(), "abnormal")
	if e := gs.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("timeout on waiting child")
	}

	if e := p.WaitWithTimeout(1 * time.Second); e != nil {
		t.Fatal("timeout on waiting application stopping")
	}

	if e := node.WaitWithTimeout(1 * time.Second); e != nil {
		t.Fatal("node shouldn't be alive here")
	}

	if node.IsAlive() {
		t.Fatal("node shouldn't be alive here")
	}

}

func TestApplicationTypeTransient(t *testing.T) {
	fmt.Printf("\n=== Test Application type Transient\n")
	fmt.Printf("\nStarting node nodeTestAplicationTypeTransient@localhost:")
	ctx := context.Background()
	node := CreateNodeWithContext(ctx, "nodeTestApplicationTypeTransient@localhost", "cookies", NodeOptions{})
	if node == nil {
		t.Fatal("can't start node")
	} else {
		fmt.Println("OK")
	}

	fmt.Printf("Starting application...")
	app1 := &testApplication{}
	app2 := &testApplication{}
	lifeSpan := time.Duration(0)

	if err := node.ApplicationLoad(app1, lifeSpan, "testapp1", "testAppGS1"); err != nil {
		t.Fatal(err)
	}

	if err := node.ApplicationLoad(app2, lifeSpan, "testapp2", "testAppGS2"); err != nil {
		t.Fatal(err)
	}

	p1, e1 := node.ApplicationStartTransient("testapp1")
	if e1 != nil {
		t.Fatal(e1)
	}

	p2, e2 := node.ApplicationStartTransient("testapp2")
	if e2 != nil {
		t.Fatal(e2)
	}

	fmt.Println("OK")

	fmt.Printf("stopping testAppGS1 with 'normal' reason (shouldn't affect testAppGS2)...")
	gs := node.GetProcessByName("testAppGS1")
	gs.Exit(gs.Self(), "normal")
	if e := gs.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal(e)
	}

	if e := p1.WaitWithTimeout(100 * time.Millisecond); e != ErrTimeout {
		t.Fatal("application testapp1 should be alive here")
	}

	p1.Kill()

	p2.WaitWithTimeout(100 * time.Millisecond)
	if !p2.IsAlive() {
		t.Fatal("testAppGS2 should be alive here")
	}

	if !node.IsAlive() {
		t.Fatal("node should be alive here")
	}

	fmt.Println("OK")

	fmt.Println("starting application testapp1")
	p1, e1 = node.ApplicationStartTransient("testapp1")
	if e1 != nil {
		t.Fatal(e1)
	}
	fmt.Println("OK")

	fmt.Printf("stopping testAppGS1 with 'abnormal' reason (node will shotdown)...")
	gs = node.GetProcessByName("testAppGS1")
	gs.Exit(gs.Self(), "abnormal")

	if e := gs.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal(e)
	}

	if e := p1.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("testapp1 shouldn't be alive here")
	}

	if e := p2.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("testapp2 shouldn't be alive here")
	}

	if e := node.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("node shouldn't be alive here")
	}
}

func TestApplicationTypeTemporary(t *testing.T) {
	fmt.Printf("\n=== Test Application type Temporary\n")
	fmt.Printf("\nStarting node nodeTestAplicationStop@localhost:")
	ctx := context.Background()
	node := CreateNodeWithContext(ctx, "nodeTestApplicationStop@localhost", "cookies", NodeOptions{})
	if node == nil {
		t.Fatal("can't start node")
	} else {
		fmt.Println("OK")
	}
	fmt.Printf("Starting application...")
	app := &testApplication{}
	lifeSpan := time.Duration(0)
	if err := node.ApplicationLoad(app, lifeSpan, "testapp"); err != nil {
		t.Fatal(err)
	}

	p, e := node.ApplicationStart("testapp") // default start type is Temporary
	if e != nil {
		t.Fatal(e)
	}

	gs := node.GetProcessByName("testAppGS")
	gs.Exit(p.Self(), "normal")
	if e := gs.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal(e)
	}

	if e := node.WaitWithTimeout(100 * time.Millisecond); e != ErrTimeout {
		t.Fatal("node should be alive here")
	}

	if !node.IsAlive() {
		t.Fatal("node should be alive here")
	}

	node.Stop()
}

func TestApplicationStop(t *testing.T) {
	fmt.Printf("\n=== Test Application stopping\n")
	fmt.Printf("\nStarting node nodeTestAplicationTypeTemporary@localhost:")
	ctx := context.Background()
	node := CreateNodeWithContext(ctx, "nodeTestApplicationTypeTemporary@localhost", "cookies", NodeOptions{})
	if node == nil {
		t.Fatal("can't start node")
	} else {
		fmt.Println("OK")
	}
	fmt.Printf("Starting applications testapp1, testapp2...")
	lifeSpan := time.Duration(0)
	app := &testApplication{}
	if e := node.ApplicationLoad(app, lifeSpan, "testapp1", "testAppGS1"); e != nil {
		t.Fatal(e)
	}

	app1 := &testApplication{}
	if e := node.ApplicationLoad(app1, lifeSpan, "testapp2", "testAppGS2"); e != nil {
		t.Fatal(e)
	}

	p1, e1 := node.ApplicationStartPermanent("testapp1")
	if e1 != nil {
		t.Fatal(e1)
	}
	p2, e2 := node.ApplicationStartPermanent("testapp2")
	if e2 != nil {
		t.Fatal(e2)
	}
	fmt.Println("OK")

	// case 1: stopping via node.ApplicatoinStop
	fmt.Printf("stopping testapp1 via node.ApplicationStop (shouldn't affect testapp2) ...")
	node.ApplicationStop("testapp1")
	if e := p1.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal("can't stop application via node.ApplicationStop")
	}

	if !p2.IsAlive() {
		t.Fatal("testapp2 should be alive here")
	}

	if !node.IsAlive() {
		t.Fatal("node should be alive here")
	}

	fmt.Println("OK")

	// case 2: stopping via process.Exit
	fmt.Printf("starting application testapp1 ...")
	p1, e1 = node.ApplicationStartPermanent("testapp1")
	if e1 != nil {
		t.Fatal(e1)
	}
	fmt.Println("OK")

	fmt.Printf("stopping testapp1 via process.Exit (shouldn't affect testapp2)...")
	p1.Exit(p1.Self(), "normal")
	if e := p1.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if !p2.IsAlive() {
		t.Fatal("testapp2 should be alive here")
	}

	if !node.IsAlive() {
		t.Fatal("node should be alive here")
	}
	fmt.Println("OK")

	// case 3: stopping via process.Kill
	fmt.Printf("starting application testapp1 ...")
	p1, e1 = node.ApplicationStartPermanent("testapp1")
	if e1 != nil {
		t.Fatal(e1)
	}
	fmt.Println("OK")

	fmt.Printf("stopping testapp1 via process.Kill (shouldn't affect testapp2)...")
	p1.Kill()
	if e := p1.WaitWithTimeout(100 * time.Millisecond); e != nil {
		t.Fatal(e)
	}

	if !p2.IsAlive() {
		t.Fatal("testapp2 should be alive here")
	}

	if !node.IsAlive() {
		t.Fatal("node should be alive here")
	}

}
