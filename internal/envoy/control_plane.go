package envoy

import (
	"context"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	testv3 "github.com/envoyproxy/go-control-plane/pkg/test/v3"

	"acnodal.io/egw-ws/internal/model"
)

var (
	cache cachev3.SnapshotCache
	l     Logger
)

func UpdateModel(nodeID string, service model.Service, endpoints []model.Endpoint) error {
	snapshot := ServiceToSnapshot(service, endpoints)
	return updateSnapshot(nodeID, snapshot)
}

func updateSnapshot(nodeID string, snapshot cachev3.Snapshot) error {
	if err := snapshot.Consistent(); err != nil {
		l.Errorf("snapshot inconsistency: %+v\n%+v", snapshot, err)
		return err
	}
	l.Debugf("will serve snapshot %+v", snapshot)

	// add the snapshot to the cache
	if err := cache.SetSnapshot(nodeID, snapshot); err != nil {
		l.Errorf("snapshot error %q for %+v", err, snapshot)
		return err
	}

	return nil
}

func LaunchControlPlane(xDSPort uint, nodeID string, debug bool) error {
	l = Logger{Debug: debug}

	// create a cache
	cache = cachev3.NewSnapshotCache(false, cachev3.IDHash{}, l)
	cbv3 := &testv3.Callbacks{Debug: debug}
	updateSnapshot(nodeID, NewSnapshot())
	srv3 := serverv3.NewServer(context.Background(), cache, cbv3)

	// run the xDS server
	runServer(context.Background(), srv3, xDSPort)

	return nil
}
