package main

import (
	"log"
	"sort"
	"strings"

	"github.com/Loyalsoldier/cn-blocked-domain/utils"
)

const LEAF = true

type domainLabel string

type domainList map[domainLabel]interface{}

func newList() *domainList {
	domainList := make(domainList)
	return &domainList
}

func (l *domainList) set(label domainLabel, value interface{}) {
	(*l)[label] = value
}

func (l *domainList) found(label domainLabel) (interface{}, bool) {
	if (*l)[label] != nil {
		return (*l)[label], true
	}
	return nil, false
}

func splitAndSortByLabelsLength(domainSlice []string) [][]string {
	sortedDomainList := make([][]string, 0, len(domainSlice))
	for _, domain := range domainSlice {
		labels := strings.Split(domain, ".")
		utils.ReverseSlice(labels)
		sortedDomainList = append(sortedDomainList, labels)
	}
	sort.SliceStable(sortedDomainList, func(i, j int) bool { return len(sortedDomainList[i]) < len(sortedDomainList[j]) })
	return sortedDomainList
}

func buildTreeAndUnique(sortedDomainList [][]string) []string {
	// Mark indexes of redundant domains in sortedDomainList for filtering purpose later
	redundantDomainID := make(map[int]bool)

	tree := newList()
	for idx, labels := range sortedDomainList {
		copiedLabels := make([]string, len(labels))
		copy(copiedLabels, labels)
		utils.ReverseSlice(copiedLabels)
		normalDomain := strings.Join(copiedLabels, ".")

		node := tree
		iterableNode := node
		for len(labels) > 0 {
			label := domainLabel(labels[0])
			labels = labels[1:]

			val, ok := node.found(label)
			if ok {
				if val == LEAF {
					redundantDomainID[idx] = true
					utils.ReverseSlice(labels)
					log.Println("Found redundant domain: ", utils.Info(normalDomain), "@", utils.Warning(strings.Join(labels, ".")))
					break
				} else {
					node = (*node)[label].(*domainList)
					continue
				}
			} else {
				if len(labels) == 0 {
					node.set(label, LEAF)
				} else {
					temp := newList()
					node.set(label, temp)
					node = temp
				}
			}
		}
		tree = iterableNode
	}

	// Remove redundant domains and build slice of remaining domains
	domainListSlice := make([]string, 0, len(sortedDomainList))
	for idx, labels := range sortedDomainList {
		if !redundantDomainID[idx] {
			utils.ReverseSlice(labels)
			domain := strings.Join(labels, ".")
			domainListSlice = append(domainListSlice, domain)
		}
	}
	return domainListSlice
}
