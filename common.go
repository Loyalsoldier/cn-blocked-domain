package main

import (
	"log"
	"strings"

	"github.com/Loyalsoldier/cn-blocked-domain/utils"
)

type sortableSlice []string

func (r sortableSlice) Len() int {
	return len(r)
}

func (r sortableSlice) Less(i, j int) bool {
	return len(strings.Split(r[i], ".")) < len(strings.Split(r[j], "."))
}

func (r sortableSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func buildTreeAndUnique(sortedDomainList []string) []string {
	tree := newList()
	remainList := make([]string, 0, len(sortedDomainList))

	for _, domain := range sortedDomainList {
		parts := strings.Split(domain, ".")
		leafIdx, isInserted, err := tree.Insert(parts)

		if err != nil {
			log.Println(utils.Fatal("[Error]"), "check domain", utils.Info(domain), "for redundancy.")
			continue
		}
		if !isInserted {
			redundantParts := make([]string, 0, len(parts))
			for i := 0; i <= leafIdx; i++ {
				redundantParts = append(redundantParts, parts[i])
			}
			redundantStr := strings.Join(redundantParts, ".")
			log.Println("Found redundant domain:", utils.Info(domain), "@", utils.Warning(redundantStr))
			continue
		}
		remainList = append(remainList, domain)
	}

	return remainList
}
