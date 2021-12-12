package main

import (
	"bitbucket.org/sjbog/go-dbscan"
)

type MediaCluster struct {
	clusters int
	entries  []Media
}

func (clusters *MediaCluster) ClusterSize() int {
	return clusters.clusters
}

/**
 * Apply DBSCAN clustering to a set of media, based on their creation times. Apply this to all
 * files present.
 */
func ClusterMedia(epsilon float64, minPoints int, library []*Media) *MediaCluster {
	// create the clusterer
	var clusterer = dbscan.NewDBSCANClusterer(epsilon, minPoints)
	clusterer.AutoSelectDimension = false
	clusterer.SortDimensionIndex = 0

	var data = make([]dbscan.ClusterablePoint, len(library))
	var mediaDict = make(map[string]Media)

	for idx, media := range library {
		mediaDict[media.source] = *media

		// create a named point, with the file as the name and the mtime as a
		// dimension it is clustered along
		data[idx] = &dbscan.NamedPoint{
			Name:  media.source,
			Point: []float64{float64(media.GetCreationTime())},
		}
	}

	// cluster, and restructure the data for use later
	clusters := clusterer.Cluster(data)
	labelledMedia := make([]Media, 0)

	for clusterId, cluster := range clusters {
		clusterList := make([]Media, len(cluster))

		for idx, point := range cluster {
			fpath := point.(*dbscan.NamedPoint).Name
			media := mediaDict[fpath]
			media.clusterId = clusterId

			clusterList[idx] = media
		}

		labelledMedia = append(labelledMedia, clusterList...)
	}

	return &MediaCluster{
		clusters: len(clusters),
		entries:  labelledMedia,
	}
}
