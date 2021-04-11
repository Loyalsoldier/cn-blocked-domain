package main

import (
	"log"
	"strings"

	"github.com/Loyalsoldier/cn-blocked-domain/utils"
)

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
