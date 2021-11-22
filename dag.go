package arangodag

import (
	"context"
	"errors"
	"github.com/arangodb/go-driver"
)

// DAG implements the data structure of the DAG.
type DAG struct {
	db       driver.Database
	vertices driver.Collection
	edges    driver.Collection
	client   driver.Client
}

// NewDAG creates / initializes a new DAG.
func NewDAG(dbName, vertexCollName, edgeCollName string, client driver.Client) (*DAG, error) {

	// use or create database
	var db driver.Database
	exists, err := client.DatabaseExists(context.Background(), dbName)
	if err != nil {
		return nil, err
	}
	if exists {
		db, err = client.Database(context.Background(), dbName)
	} else {
		db, err = client.CreateDatabase(context.Background(), dbName, nil)
	}
	if err != nil {
		return nil, err
	}

	// use or create vertex collection
	var vertices driver.Collection
	exists, err = db.CollectionExists(context.Background(), vertexCollName)
	if err != nil {
		return nil, err
	}
	if exists {
		vertices, err = db.Collection(context.Background(), vertexCollName)
	} else {
		vertices, err = db.CreateCollection(context.Background(), vertexCollName, nil)
	}
	if err != nil {
		return nil, err
	}

	// use or create edge collection
	var edges driver.Collection
	exists, err = db.CollectionExists(context.Background(), edgeCollName)
	if err != nil {
		return nil, err
	}
	if exists {
		edges, err = db.Collection(context.Background(), edgeCollName)
	} else {
		options := &driver.CreateCollectionOptions{Type: driver.CollectionTypeEdge}
		edges, err = db.CreateCollection(context.Background(), edgeCollName, options)
	}
	if err != nil {
		return nil, err
	}

	return &DAG{db: db, vertices: vertices, edges: edges, client: client}, nil
}

// AddVertex adds the given vertex to the DAG and returns its key.
//
// If the given vertex contains a `_key` field, this will be used as key. A new
// key will be created otherwise.
//
// AddVertex prevents duplicate keys.
func (d *DAG) AddVertex(vertex interface{}) (string, error) {

	ctx := driver.WithQueryCount(context.Background())
	meta, err := d.vertices.CreateDocument(ctx, vertex)
	if err != nil {
		return "", err
	}
	return meta.Key, nil
}

// GetVertex returns the vertex with the given key.
func (d *DAG) GetVertex(key string, vertex interface{}) error {
	_, err := d.getVertex(key, vertex)
	if err != nil {
		return err
	}
	return nil
}

func (d *DAG) getVertex(key string, vertex interface{}) (string, error) {
	ctx := context.Background()
	meta, err := d.vertices.ReadDocument(ctx, key, vertex)
	if err != nil {
		return "", err
	}
	return string(meta.ID), nil
}

// GetOrder returns the number of vertices in the graph.
func (d *DAG) GetOrder() (uint64, error) {
	count, err := d.vertices.Count(context.Background())
	if err != nil {
		return 0, err
	}
	return uint64(count), nil

}

type myEdge struct {
	From string `json:"_from"`
	To   string `json:"_to"`
}

// AddEdge adds an edge from src to dst.
//
// AddEdge requires that src and dst exist. AddEdge prevents duplicate edges.
func (d *DAG) AddEdge(src, dst string) error {

	// ensure vertices exist
	srcId, errSrc := d.getVertex(src, nil)
	if errSrc != nil {
		return errSrc
	}
	dstId, errDst := d.getVertex(dst, nil)
	if errDst != nil {
		return errDst
	}

	// prevent duplicate edge
	id, errEdge := d.getEdgeId(srcId, dstId)
	if errEdge != nil {
		return errEdge
	}
	if id != "" {
		return errors.New("duplicate edge")
	}

	// prevent loops
	path, errSrc := d.getShortestPath(dstId, srcId)
	if errSrc != nil {
		return errSrc
	}
	if path != nil {
		if len(path) == 1 {
			return errors.New("self loop")
		}
		return errors.New("loop")
	}

	// add edge
	ctx := context.Background()
	_, err := d.edges.CreateDocument(ctx, myEdge{srcId, dstId})
	if err != nil {
		return err
	}
	return nil
}

// IsEdge returns true, if an edge from src to dst exists.
func (d *DAG) IsEdge(src, dst string) (bool, error) {
	srcId, errSrc := d.getVertex(src, nil)
	if errSrc != nil {
		return false, errSrc
	}
	dstId, errDst := d.getVertex(dst, nil)
	if errDst != nil {
		return false, errDst
	}
	id, err := d.getEdgeId(srcId, dstId)
	if err != nil {
		return false, err
	}
	if id == "" {
		return false, nil
	}
	return true, nil
}

func (d *DAG) getEdgeId(srcId, dstId string) (string, error) {
	ctx := context.Background()
	query := "FOR d IN @@collection FILTER d._from == @from AND d._to == @to RETURN d"
	bindVars := map[string]interface{}{
		"@collection": d.edges.Name(),
		"from":        srcId,
		"to":          dstId,
	}

	cursor, err := d.db.Query(ctx, query, bindVars)
	if err != nil {
		return "", err
	}
	defer cursor.Close()
	var doc myEdge
	meta, err := cursor.ReadDocument(ctx, &doc)
	if driver.IsNoMoreDocuments(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(meta.ID), nil
}

// GetShortestPath returns the shortest path between src and dst. GetShortestPath returns nil if
// there is no such path.
func (d *DAG) GetShortestPath(src, dst string) ([]string, error) {
	srcId, errSrc := d.getVertex(src, nil)
	if errSrc != nil {
		return nil, errSrc
	}
	dstId, errDst := d.getVertex(dst, nil)
	if errDst != nil {
		return nil, errDst
	}
	path, errPath := d.getShortestPath(srcId, dstId)
	if errPath != nil {
		return nil, errPath
	}
	if path == nil {
		return nil, nil
	}
	result := make([]string, len(path))
	for i, x := range path {
		result[i] = x.Key
	}
	return result, nil
}

func (d *DAG) getShortestPath(srcId, dstId string) ([]driver.DocumentMeta, error) {
	var result []driver.DocumentMeta

	ctx := context.Background()
	query := "FOR v IN OUTBOUND SHORTEST_PATH @from TO @to @@collection RETURN v"
	bindVars := map[string]interface{}{
		"@collection": d.edges.Name(),
		"from":        srcId,
		"to":          dstId,
	}

	cursor, err := d.db.Query(ctx, query, bindVars)
	if err != nil {
		return result, err
	}
	defer cursor.Close()
	for {
		var doc myEdge
		meta, err := cursor.ReadDocument(ctx, &doc)
		if driver.IsNoMoreDocuments(err) {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, meta)
	}
	return result, nil
}

// GetSize returns the number of edges in the graph.
func (d *DAG) GetSize() (uint64, error) {
	count, err := d.edges.Count(context.Background())
	if err != nil {
		return 0, err
	}
	return uint64(count), nil
}

// GetLeaves returns the leaves of the DAG.
func (d *DAG) GetLeaves() ([]string, error) {
	leaves, errLeaves := d.getLeaves()
	if errLeaves != nil {
		return nil, errLeaves
	}
	if leaves == nil {
		return nil, nil
	}
	result := make([]string, len(leaves))
	for i, x := range leaves {
		result[i] = x.Key
	}
	return result, nil
}

func (d *DAG) getLeaves() ([]driver.DocumentMeta, error) {

	var result []driver.DocumentMeta

	ctx := context.Background()
	query := "FOR v IN @@vertexCollection " +
		"FILTER LENGTH(FOR vv IN 1..1 OUTBOUND v @@edgeCollection LIMIT 1 RETURN 1) == 0 " +
		"RETURN v"
	bindVars := map[string]interface{}{
		"@vertexCollection": d.vertices.Name(),
		"@edgeCollection":   d.edges.Name(),
	}

	cursor, err := d.db.Query(ctx, query, bindVars)
	if err != nil {
		return result, err
	}
	defer cursor.Close()
	var vertex driver.DocumentMeta
	for {
		meta, err := cursor.ReadDocument(ctx, &vertex)
		if driver.IsNoMoreDocuments(err) {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, meta)
	}
	return result, nil
}

// GetRoots returns the roots of the DAG.
func (d *DAG) GetRoots() ([]string, error) {
	leaves, errLeaves := d.getRoots()
	if errLeaves != nil {
		return nil, errLeaves
	}
	if leaves == nil {
		return nil, nil
	}
	result := make([]string, len(leaves))
	for i, x := range leaves {
		result[i] = x.Key
	}
	return result, nil
}

type myKey struct {
	Key string `json:"_key,omitempty"`
}

func (d *DAG) GetRootsWalker() (<-chan string, <-chan error, chan<- bool) {

	chanRoots := make(chan string)
	chanErrors := make(chan error)
	chanSignal := make(chan bool, 1)

	go func() {
		defer close(chanErrors)
		defer close(chanRoots)
		ctx := context.Background()
		query := "FOR v IN @@vertexCollection " +
			"FILTER LENGTH(FOR vv IN 1..1 INBOUND v @@edgeCollection LIMIT 1 RETURN 1) == 0 " +
			"RETURN v"
		bindVars := map[string]interface{}{
			"@vertexCollection": d.vertices.Name(),
			"@edgeCollection":   d.edges.Name(),
		}
		cursor, err := d.db.Query(ctx, query, bindVars)
		if err != nil {
			chanErrors <- err
			return
		}
		defer cursor.Close()

		var key myKey
		for {
			_, errRead := cursor.ReadDocument(ctx, &key)
			if driver.IsNoMoreDocuments(errRead) {
				return
			}
			if errRead != nil {
				chanErrors <- errRead
				continue
			}
			select {
			case <-chanSignal:
				return
			default:
				chanRoots <- key.Key
			}
		}
	}()

	return chanRoots, chanErrors, chanSignal
}

func (d *DAG) getRoots() ([]driver.DocumentMeta, error) {

	var result []driver.DocumentMeta

	ctx := context.Background()
	query := "FOR v IN @@vertexCollection " +
		"FILTER LENGTH(FOR vv IN 1..1 INBOUND v @@edgeCollection LIMIT 1 RETURN 1) == 0 " +
		"RETURN v"
	bindVars := map[string]interface{}{
		"@vertexCollection": d.vertices.Name(),
		"@edgeCollection":   d.edges.Name(),
	}

	cursor, err := d.db.Query(ctx, query, bindVars)
	if err != nil {
		return result, err
	}
	defer cursor.Close()
	var vertex driver.DocumentMeta
	for {
		meta, err := cursor.ReadDocument(ctx, &vertex)
		if driver.IsNoMoreDocuments(err) {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, meta)
	}
	return result, nil
}

// WalkFunc is the type expected by WalkAncestors.
type WalkFunc func(key string, err error) error

// WalkAncestors walks all ancestors of key and applies the function fn. If dfs
// is set to true, the traversal will be executed depth-first (default breadth
// first).
func (d *DAG) WalkAncestors(key string, fn WalkFunc, dfs bool) error {

	// get the id of the vertex
	id, errVertex := d.getVertex(key, nil)
	if errVertex != nil {
		return errVertex
	}

	// compute query options
	uniqueVertices := "global"
	order := "bfs"
	if dfs {
		order = "dfs"
		uniqueVertices = "none"
	}

	// compute the query
	query := "FOR v IN 1..10000 INBOUND @from @@collection OPTIONS {order: @order, uniqueVertices: @uniqueVertices}" +
		"RETURN DISTINCT v"
	bindVars := map[string]interface{}{
		"@collection":    d.edges.Name(),
		"from":           id,
		"order":          order,
		"uniqueVertices": uniqueVertices,
	}

	// execute the query
	ctx := context.Background()
	cursor, errQuery := d.db.Query(ctx, query, bindVars)
	if errQuery != nil {
		return errQuery
	}
	defer cursor.Close()

	// iterate query results
	var vertex driver.DocumentMeta
	for {
		meta, errRead := cursor.ReadDocument(ctx, &vertex)
		if driver.IsNoMoreDocuments(errRead) {
			break
		}
		if errRead != nil {
			return errRead
		}

		// apply function
		errFn := fn(meta.Key, nil)
		if errFn != nil {
			return errFn
		}
	}
	return nil
}

/*
func (d *DAG) getChildCount(id driver.DocumentID) (uint64, error) {
	// TODO: use bind variables
	ctx := driver.WithQueryCount(context.Background())
	query := fmt.Sprintf("FOR d IN %s FILTER d._from == %s RETURN d", d.edges.Name(), id)
	db := d.edges.Database()
	cursor, err := db.Query(ctx, query, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close()
	return uint64(cursor.Count()), nil
}

func (d *DAG) GetRoots() (map[string]struct{}, error) {
	// TODO: use bind variables
	query := fmt.Sprintf("FOR d IN %s RETURN d", d.vertices.Name())
	db := d.vertices.Database()
	cursor, err := db.Query(nil, query, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	leaves := make(map[string]struct{})
	var i map[string]interface{}
	for {
		meta, err := cursor.ReadDocument(nil, &i)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, err
		}

		childCount, err := d.getParentCount(meta.ID)
		if err != nil {
			return nil, err
		}
		if childCount == 0 {
			leaves[meta.Key] = struct{}{}
		}
	}
	return leaves, nil
}

func (d *DAG) getParentCount(id driver.DocumentID) (uint64, error) {
	// TODO: use bind variables
	ctx := driver.WithQueryCount(context.Background())
	query := fmt.Sprintf("FOR d IN %s FILTER d._to == %s RETURN d", d.edges.Name(), id)
	db := d.edges.Database()
	cursor, err := db.Query(ctx, query, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close()
	return uint64(cursor.Count()), nil
}

func (d *DAG) GetVertices() (map[string]struct{}, error) {
	// TODO: implement paging
	query := fmt.Sprintf("FOR d IN %s RETURN d", d.vertices.Name())
	db := d.vertices.Database()
	cursor, err := db.Query(nil, query, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	vertices := make(map[string]struct{})
	var i map[string]interface{}
	for {
		meta, err := cursor.ReadDocument(nil, &i)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, err
		}
		vertices[meta.Key] = struct{}{}
	}
	return vertices, nil
}



func (d *DAG) DeleteVertex(key string) error {
	panic("implement me")
}

func (d *DAG) AddEdge(srcKey, dstKey string) error {
	panic("implement me")
}

func (d *DAG) IsEdge(srcKey, dstKey string) (bool, error) {
	panic("implement me")
}

func (d *DAG) DeleteEdge(srcKey, dstKey string) error {
	panic("implement me")
}

func (d *DAG) GetParents(key string) (map[string]struct{}, error) {
	panic("implement me")
}

func (d *DAG) GetChildren(key string) (map[string]struct{}, error) {
	panic("implement me")
}

func (d *DAG) GetAncestors(key string) (map[string]struct{}, error) {
	panic("implement me")
}

func (d *DAG) GetOrderedAncestors(key string) ([]string, error) {
	panic("implement me")
}

func (d *DAG) AncestorsWalker(key string) (chan string, chan bool, error) {
	panic("implement me")
}

func (d *DAG) GetDescendants(key string) (map[string]struct{}, error) {
	panic("implement me")
}

func (d *DAG) GetOrderedDescendants(key string) ([]string, error) {
	panic("implement me")
}

func (d *DAG) DescendantsWalker(v string) (chan string, chan bool, error) {
	panic("implement me")
}

func (d *DAG) String() string {
	panic("implement me")
}
*/
