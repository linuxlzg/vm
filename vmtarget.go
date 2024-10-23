package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"context"
	"flag"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/gorilla/mux"
)

// 定义结构体来解析 JSON 响应
type Target struct {
	Labels             map[string]string `json:"labels"`
	LastScrape        string             `json:"lastScrape"`
	LastScrapeDuration float64            `json:"lastScrapeDuration"`
	Health            string             `json:"health"`
	ScrapePool        string             `json:"scrapePool"`
	ScrapeUrl         string             `json:"scrapeUrl"`
}

type TargetsResponse struct {
	Status string   `json:"status"`
	Data   struct {
		Active []Target `json:"activeTargets"`
		Dropped []Target `json:"droppedTargets"`
	} `json:"data"`
}

// Prometheus 指标
var (
	targetsHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vmagent_target_health",
			Help: "Health status of targets for each vmagent.",
		},
		[]string{"vmagent", "instance"},
	)
)

func init() {
	// 注册指标
	prometheus.MustRegister(targetsHealth)
}

func fetchTargets(vmagent string) {
	url := fmt.Sprintf("http://%s/api/v1/targets", vmagent)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching targets from %s: %v\n", vmagent, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: received non-200 response status from %s: %s\n", vmagent, resp.Status)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body from %s: %v\n", vmagent, err)
		return
	}

	var targetsResponse TargetsResponse
	if err := json.Unmarshal(body, &targetsResponse); err != nil {
		fmt.Printf("Error parsing JSON from %s: %v\n", vmagent, err)
		return
	}

	// 更新指标
	for _, target := range targetsResponse.Data.Active {
		health := 0.0
		if target.Health == "up" {
			health = 1.0
		}
		targetsHealth.WithLabelValues(vmagent, target.Labels["instance"]).Set(health)
	}
}

func getVmagentList() []string {

	// 创建 Kubernetes 客户端配置
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error getting in-cluster config: %s\n", err.Error())
		os.Exit(1)
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %s\n", err.Error())
		os.Exit(1)
	}

	// 指定命名空间
	namespace := "monitor"

	// 获取 Pod 列表
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting pods in namespace %s: %s\n", namespace, err.Error())
		os.Exit(1)
	}

	// 打印 Pod 信息
	for _, pod := range pods.Items {
		// 获取 Pod 的 IP 地址
		podIP := pod.Status.PodIP
        var vmagents string
		vmagents = += PodIP + ":8429, "
		
	}
	return vmagents
}

func scrapeTargets() {
	vmagents := getVmagentList()
	for _, vmagent := range vmagents {
		fetchTargets(vmagent)
	}
}

func main() {
	// 设置定时任务
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	go func() {
		for {
			scrapeTargets()
			<-ticker.C
		}
	}()

	// 设置 HTTP 路由
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())

	// 启动 HTTP 服务器
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}