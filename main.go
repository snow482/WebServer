package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)
// Coin входная структура
type Coin struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price_24h" `
	Volume float64 `json:"volume_24h"`
	LastTrade float64 `json:"last_trade_price"`
}

type CoinInfo struct {
	Price  float64 `json:"price" `
	Volume float64 `json:"volume"`
	LastTrade float64 `json:"last_trade"`
}

type CI struct {
	Sbl string `json:"symbol"`
}

// CoinGet базовая структура ...
type CoinGet struct {
	url string
	coinChannel *chan Coin
	client http.Client
}
// Construct метод конструктор, иницилизация полей базовой структуры
// chan - своего рода потокобезопасная очередь
func (c *CoinGet) Construct(ch *chan Coin){
	c.url = "https://api.blockchain.com/v3/exchange/tickers"
	c.coinChannel = ch
	c.client = http.Client{}
}

// Work рабочий цикл (типа while true)
func (c *CoinGet) Work(){
	for now := range time.Tick(time.Second * 30) {
		// вывод в консоль, каждый заданый тик
		fmt.Printf("UPDATE DATA AT {%s}",now)
		//Get запрос по url
		res, err := c.client.Get(c.url)
		if err != nil {
			fmt.Printf("Request to {%s} failed {Error : %s}",c.url,err)
			continue
		}

		// закрытие body и очистка памяти перед выходом из цикла
		defer res.Body.Close()
		// считываем побайтово внутренности body
		bytes,errRead := ioutil.ReadAll(res.Body)
		if errRead != nil{
			continue
		}

		// создаем массив coins структур типа Coin
		var coins []Coin
		// разбираем (десериализируем) json побайтово в coins
		_ = json.Unmarshal(bytes,&coins)
		// проходим по coins
		for _,co := range coins {
			// передача значения co в указатель на канал для поставки данных в горутину
			*c.coinChannel <- co
		}

	}
}
// ставим mutex и создаем map
type DB struct {
	mu    sync.Mutex
	mapDb map[string] Coin
}
// GetCoin метод возврата значения ключа
// вызов Lock()/Unlock(), чтобы у горутин не было гонок
func (db *DB) GetCoin(id string) Coin {
	// отложенный Unlock()
	defer db.mu.Unlock()
	db.mu.Lock()
	return db.mapDb[id]
}

func (db *DB) UpdateCoin(coin Coin)  {
	defer db.mu.Unlock()
	db.mu.Lock()
	db.mapDb[coin.Symbol] = coin
}

/*func (db *DB) GetAllData() []Coin  {
	defer db.mu.Unlock()
	db.mu.Lock()
	res := make([]Coin , 0 , len(db.mapDb))
	for _ , coin := range db.mapDb {
		res = append(res,coin)
	}

	return res
}*/

func (db *DB) GetAndConvertData() map[string][]CoinInfo {
	defer db.mu.Unlock()
	db.mu.Lock()
	res:= make([]Coin, 0, len(db.mapDb))
	for _ , coin := range db.mapDb {
		res = append(res,coin)
	}

	newRes := make(map[string][]CoinInfo, len(res))

	for _, value := range res {
		str := CoinInfo {
			Price: value.Price,
			Volume: value.Volume,
			LastTrade: value.LastTrade,
		}
		newRes[value.Symbol] = append(newRes[value.Symbol], str)
	}

	return newRes
}


func main(){
	// создаем канал и вычитываем по 100к элементов
	channel := make(chan Coin, 100000)
	get := CoinGet{}
	get.Construct(&channel)

	db := DB{mapDb: make(map[string]Coin)}

	// вызов горутины
	go func(cGet *CoinGet) {
		cGet.Work()
	}(&get)

	go func(db *DB, ch * chan Coin){
		for true {
			coin := <- *ch
			db.UpdateCoin(coin)
		}
	}(&db,&channel)

	http.HandleFunc("/api/", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			//coins := db.GetAllData()
			coins := db.GetAndConvertData()
			jsonArray,_ := json.Marshal(coins)
			writer.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(writer,string(jsonArray)); err != nil {
				log.Fatal(err)
			}
		}
	})

	log.Fatal(http.ListenAndServe("localhost:8080",nil))
}




