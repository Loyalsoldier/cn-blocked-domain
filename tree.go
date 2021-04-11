package main

import "errors"

type node struct {
	leaf     bool
	children map[string]*node
}

func newNode() *node {
	return &node{
		leaf:     false,
		children: make(map[string]*node),
	}
}

func (n *node) getChild(s string) *node {
	return n.children[s]
}

func (n *node) hasChild(s string) bool {
	return n.getChild(s) != nil
}

func (n *node) addChild(s string, child *node) {
	n.children[s] = child
}

func (n *node) isLeaf() bool {
	return n.leaf
}

type domainList struct {
	root *node
}

func newList() *domainList {
	return &domainList{
		root: newNode(),
	}
}

func (t *domainList) Insert(parts []string) (int, bool, error) {
	if len(parts) == 0 {
		return 0, false, errors.New("empty domain")
	}

	node := t.root
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		if node.isLeaf() {
			return i, false, nil
		}
		if !node.hasChild(part) {
			node.addChild(part, newNode())
			if i == 0 {
				node.getChild(part).leaf = true
				return 0, true, nil
			}
		}
		node = node.getChild(part)
	}
	return 0, false, nil
}
