package main

import (
	"bitbucket.org/sjbog/go-dbscan"
)

/**
 *
 */
type MediaCluster struct {
	clusters int
	entries  []Media
}

/**
 * Return the number of clusters to be created by DBSCAN
 *
 */
func (clusters *MediaCluster) ClusterSize() int {
	return clusters.clusters
}

/**
 * Apply DBSCAN clustering to a set of media, based on their creation times. Apply this to all
 * files present.
 */
func ClusterMedia(epsilon float64, minPoints int, library *MediaList) *MediaCluster {
	// create the clusterer
	var clusterer = dbscan.NewDBSCANClusterer(epsilon, minPoints)
	clusterer.AutoSelectDimension = false
	clusterer.SortDimensionIndex = 0

	// create a clusterable data-array
	var data = make([]dbscan.ClusterablePoint, library.Size())
	var mediaDict = make(map[string]Media)

	for idx, media := range library.Values() {
		mediaDict[media.source] = *media

		// create a named point, with the file as the name and the mtime as a
		// dimension it is clustered along
		data[idx] = &dbscan.NamedPoint{
			Name:  media.source,
			Point: []float64{float64(media.GetCreationTime())},
		}
	}

	// cluster the media, and restructure the data for use later
	clusters := clusterer.Cluster(data)
	labelledMedia := make([]Media, 0)

	for clusterId, cluster := range clusters {
		clusterList := make([]Media, len(cluster))

		for idx, point := range cluster {
			// associate the media with a cluster ID in a flat list
			fpath := point.(*dbscan.NamedPoint).Name
			media := mediaDict[fpath]
			media.clusterId = clusterId

			clusterList[idx] = media
		}

		labelledMedia = append(labelledMedia, clusterList...)
	}

	// return number of clusters, and the clustered media-entries
	return &MediaCluster{
		clusters: len(clusters),
		entries:  labelledMedia,
	}
}

/**
 *
 */
func (cluster *MediaCluster) GetByPrefix(media *Media) []*Media {
	prefix := media.GetPrefix()

	matches := []*Media{}

	for _, candidate := range cluster.entries {
		if candidate.GetPrefix() == prefix {
			matches = append(matches, &candidate)
		}
	}

	return matches
}
