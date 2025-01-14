package news

import (
	"app/shared/database/mysql"
	"github.com/coocood/freecache"
	"golang.org/x/tools/container/intsets"
	"encoding/json"
	"log"
	"fmt"
)

const (
	LIMIT_POST          = 100
	LIMIT_TRENDING_NEWS = 10
	CACHE_SIZE          = 100 * 1024 * 1024
	EXPIRE_TIME         = 60 * 86400 //60 days
)

var (
	cache                               *freecache.Cache //in-memcache similar to Redis
	ProbabilityCategoryEqualCiShortTerm map[int64]float64
	TrendingNews                        []News
)

func init() {
	cache = freecache.NewCache(CACHE_SIZE)
	ProbabilityCategoryEqualCiShortTerm = make(map[int64]float64)
	TrendingNews = make([]News, 0)
}

//define news type
type News struct {
	NewsId          string  `json:"newsId,omitempty"`
	CatId           int64   `json:"catId,omitempty"`
	CountViews      int64   `json:"count,omitempty"`
	TrendingScore   float64 `json:"trendscore,omitempty"`
	SimilarityScore float64 `json:"similarity,omitempty"`
}

//count news by category for each month in year
func LoadProbabilityCategoryEqualCi() (error) {
	mapProb := make(map[int64]float64)
	query := "SELECT catId, count(*) " +
		"FROM news.news_resource WHERE sourceNews='CafeBiz' AND is_deleted = false " +
		"AND publishDate > (NOW() - INTERVAL 5 HOUR)AND publishDate < current_timestamp() " +
		"GROUP BY catId;"
	rows, err := mysql.DB2.Query(query)
	if err != nil {
		log.Println("CountNewsPeriodByCategory ", err)
		return err
	}
	defer rows.Close()

	var totalPost int64 = 0
	for rows.Next() {
		var catId, count int64
		err := rows.Scan(&catId, &count)
		if err != nil {
			log.Println(err)
		}
		mapProb[catId] = float64(count)
		totalPost = totalPost + count
	}

	for k, v := range mapProb {
		mapProb[k] = float64(v) / float64(totalPost)
	}
	return nil
}

func GetProbCatIdEqualCi(catId int64) float64 {
	if val, ok := ProbabilityCategoryEqualCiShortTerm[catId]; ok {
		return val
	}
	return 0
}

func LoadTrendingNews() {
	log.Println("Load trending news")
	query := "SELECT newsId, catId, count FROM trending_news order by insertDate DESC LIMIT ?;"
	rows, err := mysql.DB.Query(query, LIMIT_TRENDING_NEWS)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

	results := make([]News, 0)
	for rows.Next() {
		var news News
		err := rows.Scan(&news.NewsId, &news.CatId, &news.CountViews)
		if err != nil {
			log.Println(err)
			return
		}
		results = append(results, news)
	}
	// calculate trending score
	var min, max int64
	min = int64(intsets.MaxInt)
	for _, news := range results {
		if news.CountViews > max {
			max = news.CountViews
		}
		if news.CountViews < min {
			min = news.CountViews
		}
	}
	for i, news := range results {
		results[i].TrendingScore = float64(news.CountViews-min) / float64(max-min)
	}
	log.Println("Load trending news done")
	TrendingNews = results;
}

//get trending news based on number of pages views
func GetTrendingNews() ([]News, error) {
	return TrendingNews, nil
}

//get news silimarity in cache
func GetNewsSimilarity(newsId string) ([]News, error) {
	arrNews := make([]News, 0)
	value, err := cache.Get([]byte(newsId))
	fmt.Println(string(value))
	if err != nil {
		return arrNews, err
	}
	e := json.Unmarshal(value, &arrNews)
	if e != nil {
		fmt.Println(err)
	}
	return arrNews, nil
}

//pre-load newsId, list news similarity to it in cached
func LoadNewsSimilarityToCache() (error) {
	log.Println("Load news similarity to cache")
	query := "SELECT newsId, similarity FROM similarity_news;"
	rows, err := mysql.DB.Query(query)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var newsId, similarity string
		err := rows.Scan(&newsId, &similarity)
		if err != nil {
			log.Println(err)
			return err
		}
		cache.Set([]byte(newsId), []byte(similarity), EXPIRE_TIME)
	}
	log.Println("Load news similarity to cache done.")
	return nil
}

func GetLastPost(itemid string) ([]News, error) {
	var arrPost []News = make([]News, 0, LIMIT_POST)
	m := make(map[string]bool)
	rows, err := mysql.DB2.Query(
		"SELECT newsId FROM news_resource WHERE catId IN (SELECT catId FROM news_resource where newsId =?) "+
			" AND publishDate < current_timestamp() AND newsId !=?"+
			" ORDER BY publishDate DESC LIMIT ?;", itemid, itemid, LIMIT_POST)
	if err != nil {
		fmt.Println(err)
		return arrPost, err
	}

	defer rows.Close()
	for rows.Next() {
		var news News
		err := rows.Scan(&news.NewsId)
		if err != nil {
			fmt.Println(err)
		}
		m[news.NewsId] = true
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
	}

	if len(arrPost) < LIMIT_POST {
		rows, err := mysql.DB.Query("SELECT newsId FROM news_resource WHERE newsId !=? "+
			"AND publishDate < current_timestamp() ORDER BY publishDate DESC LIMIT ?", itemid, LIMIT_POST)
		if err != nil {
			fmt.Println(err)
			return arrPost, err
		}
		defer rows.Close()
		for rows.Next() {
			var news News
			err := rows.Scan(&news.NewsId)
			if err != nil {
				fmt.Println(err)
			}
			if len(m) > LIMIT_POST {
				break
			}
			m[news.NewsId] = true
		}
	}
	for k, _ := range m {
		arrPost = append(arrPost, News{NewsId: k})
	}
	return arrPost, err
}
