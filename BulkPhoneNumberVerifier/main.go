package main

import (
	"fmt"
	"log"

	//"flag"
	"bufio"
	"encoding/csv"
	"encoding/json"
	"io/ioutil"
	"strings"

	//"encoding/hex"
	"os"

	"github.com/EDDYCJY/fake-useragent"
	"github.com/PuerkitoBio/goquery"
	"github.com/nyaruka/phonenumbers"

	//"github.com/gocolly/colly"
	//"github.com/gocolly/colly/proxy"
	//"github.com/parnurzeal/gorequest"
	"crypto/md5"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	inputFile  *string
	outputFile *string
)

type (
	Proxy struct {
		ip       string
		port     string
		country  string
		used_cnt int
	}
	VerifyData struct {
		Valid                bool
		Number               string
		Local_format         string
		International_format string
		Country_prefix       string
		Country_code         string
		Country_name         string
		Location             string
		Carrier              string
		Line_type            string
	}
)

const MAX_NUMBERS_PER_THREAD = 50
const MAX_THREAD_LIMIT = 2

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// get proxies function
func get_proxies(max_limit int) []Proxy {
	// Retrieve latest proxies
	var proxies []Proxy

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// Create and modify HTTP request before sending
	//request, err := http.NewRequest("GET", "https://www.sslproxies.org/", nil)
	request, err := http.NewRequest("GET", "https://www.socks-proxy.net/", nil)

	if err != nil {
		log.Print("Cannot connect to proxy provider.")
	}
	random_agent := browser.Random()
	request.Header.Set("User-Agent", random_agent)

	// Make request
	response, err := client.Do(request)
	if err != nil {
		return []Proxy{}
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Printf("Status code error: %d %s", response.StatusCode, response.Status)
		return []Proxy{}
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Response Error")
	}
	bodyString := string(bodyBytes)

	document, err := goquery.NewDocumentFromReader(strings.NewReader(bodyString))
	if err != nil {
		log.Print("Error loading HTTP response body.")
	}

	// Find all links and process them with the function
	// defined earlier
	var rows [][]string
	document.Find("#proxylisttable").Each(func(i int, tablehtml *goquery.Selection) {
		tablehtml.Find("tbody").Each(func(i int, body *goquery.Selection) {
			body.Find("tr").EachWithBreak(func(indextr int, rowhtml *goquery.Selection) bool {
				var row []string
				rowhtml.Find("td").Each(func(indexth int, tablecell *goquery.Selection) {
					row = append(row, tablecell.Text())
				})
				var proxy Proxy
				proxy = Proxy{row[0], row[1], row[3], 0}
				if len(rows) < max_limit {
					proxies = append(proxies, proxy)
				} else {
					return false
				}
				return true
			})
		})
	})

	return proxies
}

// end - update proxies function

func localscan(InputNumber string) map[string]string {

	number, _ := phonenumbers.Parse(InputNumber, "US")

	// ... deal with err appropriately ...
	formattedNumber := phonenumbers.Format(number, phonenumbers.INTERNATIONAL)
	defaultNumber := phonenumbers.Format(number, phonenumbers.E164)
	numberObj := map[string]string{
		"input":          InputNumber,
		"default":        defaultNumber,
		"local":          "",
		"international":  formattedNumber,
		"country":        "",
		"countryCode":    "",
		"countryIsoCode": "",
		"location":       "",
		"carrier":        "",
	}
	return numberObj
}

type ConvertibleBoolean bool

func (bit ConvertibleBoolean) UnmarshalJSON(data []byte) error {
	asString := string(data)
	if asString == "1" || asString == "true" {
		bit = true
	} else if asString == "0" || asString == "false" {
		bit = false
	}
	return nil
}

func numverify(number string, proxyURLStr string) (int, VerifyData) {
	invalid_data := VerifyData{false, number, "NONE", "NONE", "NONE", "NONE", "NONE", "NONE", "NONE", "NONE"}
	//test('Running Numverify.com scan...')
	var start time.Time
	start = time.Now()

	proxyUrl, err := url.Parse("socks4://" + proxyURLStr)

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
	}

	//adding the Transport object to the http Client
	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	request, err := http.NewRequest("GET", "https://numverify.com", nil)
	if err != nil {
		log.Print("Create Request Error - Numverify.com ")
		return -2, invalid_data
	}
	random_agent := browser.Random()
	request.Header.Set("User-Agent", random_agent)

	response, err := client.Do(request)
	if err != nil {
		log.Println(err)
		current := time.Now()
		elapsed := current.Sub(start)
		log.Printf("Numverify.com is not available. %02d, %02d ", int(elapsed.Minutes()), int(elapsed.Seconds()))
		return -2, invalid_data
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Printf("Status code error: %d %s", response.StatusCode, response.Status)
		return -2, invalid_data
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	bodyString := string(bodyBytes)

	document, err := goquery.NewDocumentFromReader(strings.NewReader(bodyString))
	if err != nil {
		log.Print("Error loading HTTP response body.")
		return -2, invalid_data
	}

	requestSecret, _ := document.Find("input[name='scl_request_secret']").Attr("value")

	keydata := []byte(number + requestSecret)
	apiKey := fmt.Sprintf("%x", md5.Sum(keydata))

	verifyURL := fmt.Sprintf("https://numverify.com/php_helper_scripts/phone_api.php?secret_key=%s&number=%s", apiKey, number)
	request, err = http.NewRequest("GET", verifyURL, nil)
	if err != nil {
		log.Print("Creating Request Error with key.")
		return -2, invalid_data
	}

	request.Header.Set("Host", "numverify.com")
	request.Header.Set("User-Agent", random_agent)
	request.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	request.Header.Set("Accept-Language", "fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3")
	request.Header.Set("Accept-Encoding", "gzip, deflate, br")
	request.Header.Set("Referer", "https://numverify.com/")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("DNT", "1")
	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("Pragma", "no-cache")
	request.Header.Set("Cache-Control", "no-cache")

	// Make request
	response, err = client.Do(request)
	if err != nil {
		return -2, invalid_data
	}
	defer response.Body.Close()
	bodyBytes, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return -2, invalid_data
	}
	responseString := string(bodyBytes)

	if responseString == "Unauthorized" || response.StatusCode != 200 {
		log.Printf("Status code error: %d %s", response.StatusCode, response.Status)
		return -2, invalid_data
	}

	var data VerifyData
	err = json.Unmarshal([]byte(responseString), &data)
	if err != nil {
		log.Print("Parsing Json Error.")
		return -2, invalid_data
	}

	return 0, data
}

func scanNumber(InputNumber string, proxy Proxy) (int, VerifyData) {
	//proxy: {'https', '10.23.44.22:3324'}
	result := 0
	invalid_data := VerifyData{false, InputNumber, "NONE", "NONE", "NONE", "NONE", "NONE", "NONE", "NONE", "NONE"}
	var data VerifyData
	number := localscan(InputNumber)
	if number == nil {
		data = invalid_data
	} else {
		result, data = numverify(InputNumber, fmt.Sprintf("%s:%s", proxy.ip, proxy.port))
	}
	return result, data
}

//func scanNumbersWithProxy(phonenum_list []string, filename string){
func scanNumbersWithProxy(phonenum_list []string, filename string, wg *sync.WaitGroup) {
	var proxies []Proxy

	start := time.Now()
	var result_list []map[string]string
	for len(phonenum_list) > 0 {
		phonenum := phonenum_list[0]
		for len(proxies) == 0 {
			log.Printf("Getting Proxies =PROC= %s!", filename)
			proxies = get_proxies(100)
			if len(proxies) == 0 {
				time.Sleep(2)
			}
		}
		proxy_index := rand.Intn(len(proxies) - 1)
		proxy := proxies[proxy_index]
		proxy_addr := fmt.Sprintf("%s:%s", proxy.ip, proxy.port)
		result, data := scanNumber(phonenum, proxy)
		if result == -1 {
			log.Printf("Scanning Failed-%s is not correct format.", phonenum)
			res := map[string]string{"phone_number": phonenum, "valid": "False", "Country": "NONE", "Location": "NONE", "Carrier": "NONE", "Line type": "NONE"}
			result_list = append(result_list, res)
			phonenum_list = phonenum_list[1:] //pop 0
		} else if result == -2 { //proxy fail
			log.Printf("Scanning Failed-connection error:%s", proxy_addr)
			//remove failed proxy
			proxies = append(proxies[:proxy_index], proxies[proxy_index+1:]...) //pop
			continue
		} else if result == 0 {
			log.Printf("%s#Scanning Finished--------------%s:%t", filename, phonenum, data.Valid)
			var valid string
			if data.Valid {
				valid = "True"
			} else {
				valid = "False"
			}
			res := map[string]string{"phone_number": phonenum, "valid": valid, "Country": data.Country_name, "Location": data.Location, "Carrier": data.Carrier, "Line type": data.Line_type}
			result_list = append(result_list, res)
			phonenum_list = phonenum_list[1:] //pop 0
			proxies[proxy_index].used_cnt += 1
		}
		time.Sleep(time.Nanosecond * 10)

		//remove too many used proxy
		if proxies[proxy_index].used_cnt > 10 {
			proxies = append(proxies[:proxy_index], proxies[proxy_index+1:]...) //pop
		}

		current := time.Now()
		elapsed := current.Sub(start)
		elapsed_minu := elapsed.Minutes()
		if len(proxies) == 0 || elapsed_minu > 5 { //over 5 min
			proxies = proxies[:0]
			proxies = get_proxies(100)
		}
	}
	file, _ := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0777)
	csvWriter := csv.NewWriter(file)
	csvWriter.Write([]string{"phone_number", "valid", "Country", "Location", "Carrier", "Line type"})
	for _, res := range result_list {
		res_data := []string{}
		for _, val := range res {
			res_data = append(res_data, val)
		}
		csvWriter.Write(res_data)
	}

	file.Close()
	wg.Done()
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func init() {
	//inputFile = flag.String("input", "", "input text file contains phone numbers. A phone number for each line.")
	//outputFile = flag.String("output", "", "result csv file")
}

func main() {

	//flag.Parse()

	//phonenum_list, err := readLines(*inputFile)
	phonenum_list, err := readLines("phones.txt")
	if err != nil {
		log.Fatalf("readLines: %s", err)
	}

	total_cnt := len(phonenum_list)

	proc_cnt := Max(1, Min(MAX_THREAD_LIMIT, total_cnt/10))
	cnt_per_proc := int(total_cnt / proc_cnt)

	log.Printf("PhoneNum Cnt:%d, Proc Cnt: %d, PhoneNums Per Proc: %d", total_cnt, proc_cnt, cnt_per_proc)

	start := time.Now()

	var waitGroup sync.WaitGroup
	//waitGroup.Add(proc_cnt)
	waitGroup.Add(1)
	scanNumbersWithProxy(phonenum_list, "filename", &waitGroup)

	/*for i := 0; i < proc_cnt; i += 1 {
	    begin := i * cnt_per_proc
	    var end int
	    end = begin + cnt_per_proc
	    if i == (proc_cnt - 1){
	        end = Max(end, len(phonenum_list))
	    }
	    sub_list := phonenum_list[begin:end]

	    filename := fmt.Sprintf("proc%d-%d", begin, end)
	    go scanNumbersWithProxy(sub_list, filename, &waitGroup)
	    //time.Sleep(time.Nanosecond * 10)
	}*/

	waitGroup.Wait()

	end := time.Now()
	elapsed := end.Sub(start)
	log.Println(elapsed)

	//hours, rem = divmod(end-start, 3600)
	//minutes, seconds = divmod(rem, 60)

	//fmt.Printf("Ellapsed time: {:0>2}:{:0>2}:{:05.2f}".format(int(hours),int(minutes),seconds))
}
