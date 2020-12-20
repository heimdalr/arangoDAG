# arangodag

[![run tests](https://github.com/heimdalr/arangodag/workflows/Test/badge.svg?branch=main)](https://github.com/heimdalr/arangodag/actions?
query=workflow%3ATest+branch%3Amain)

<!--
[![codecov](https://codecov.io/gh/heimdalr/dag/branch/master/graph/badge.svg)](https://codecov.io/gh/heimdalr/dag)
[![GoDoc](https://godoc.org/github.com/heimdalr/dag?status.svg)](https://godoc.org/github.com/heimdalr/dag) 
[![Go Report Card](https://goreportcard.com/badge/github.com/heimdalr/dag)](https://goreportcard.com/report/github.com/heimdalr/dag)
[![Nutrition Facts](http://code.grevit.net/badge/O%2B%2B_S%2B%2B_I%2B_C%2B_E%2B%2B%2B_M_V%2B_PS%2B%2B_!D)](http://code.grevit.net/facts/O%2B%2B_S%2B%2B_I%2B_C%2B_E%2B%2B%2B_M_V%2B_PS%2B%2B_!D)

Implementation directed acyclic graphs (DAGs).

The implementation is fast and thread-safe. It prevents adding cycles or 
duplicates and thereby always maintains a valid DAG. The implementation caches
 descendants and ancestors to speed up subsequent calls. 


github.com/heimdalr/dag:

3.770388s to add 597871 vertices and 597870 edges
1.578741s to get descendants
0.143887s to get descendants 2nd time
0.444065s to get descendants ordered
0.000008s to get children
1.301297s to transitively reduce the graph with caches populated
2.723708s to transitively reduce the graph without caches populated
0.168572s to delete an edge from the root


"github.com/hashicorp/terraform/dag":

3.195338s to add 597871 vertices and 597870 edges
1.121812s to get descendants
1.803096s to get descendants 2nd time
3.056972s to transitively reduce the graph





## Quickstart

Running: 

``` go
package main

import (
	"fmt"
	"github.com/heimdalr/dag/drivers/memdag"
)

func main() {

	// initialize a new graph
	d := memdag.NewDAG()

	// add three vertices
	key1, _ := d.AddVertex(1)
	key2, _ := d.AddVertex(2)
	key3, _ := d.AddVertex(3)

	// add the above vertices and connect them with two edges
	_ = d.AddEdge(key1, key2)
	_ = d.AddEdge(key1, key3)

	// describe the graph
	fmt.Print(d.String())

	// describe the graph
	fmt.Print(d.String())
}
```

will result in something like:

```
DAG Vertices: 3 - Edges: 2
Vertices:
  2
  3
  1
Edges:
  1 -> 2
  1 -> 3
```
-->
