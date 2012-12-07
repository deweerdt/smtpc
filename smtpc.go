package main

import (
	"net"
	"log"
	"math/rand"
	"fmt"
	"io/ioutil"
	"flag"
	"time"
	"strings"
	"strconv"
	"encoding/base64"
)

func write(s *net.TCPConn, str string) (code int, err error, err_str string) {
	code = 200

	_, err = s.Write([]byte(str))
	if err != nil {
		return
	}

	// Oops, we're allocation a new buffer every time!
	var b = make([]byte, 1024)
	_, err = s.Read(b)
	if err != nil {
		return
	}

	err_str = string(b[0:])
	code, err = strconv.Atoi(err_str[0:3])

	return
}

var verbose bool

func close_s(s *net.TCPConn) (err error) {
	var err_str string
	var code int

	code, err, err_str = write(s, "QUIT\r\n")
	if verbose {
		log.Println("QUIT\r\n")
	}
	if err != nil {
		return
	}
	if code > 399 {
		log.Println(err_str)
	}

	s.Close()
	return
}

func connect_s(l, a *net.TCPAddr, hello string) (s *net.TCPConn, err error) {
	var err_str string
	var code int

	s, err = net.DialTCP("tcp4", l, a)
	if err != nil {
		return
	}
	// Read banner
	var b = make([]byte, 1024)
	_, err = s.Read(b)

	_, err, err_str = write(s, "EHLO "+hello+"\r\n")
	if verbose {
		log.Println("EHLO "+hello+"\r\n")
	}
	if err != nil {
		return
	}
	if code > 399 {
		log.Println(err_str)
	}

	return
}

type RoundRobin struct {
	strings       []string
	length        int
	current_index int
	randomize     bool
	range_min     int
	range_max     int
	is_random     []bool
}

func (rrs *RoundRobin) Peek() string {
	s := rrs.strings[rrs.current_index%rrs.length]
	if rrs.randomize {
		if rrs.is_random[rrs.current_index%rrs.length] {
			split := strings.Split(s, "%")
			rand := (rand.Int() % (rrs.range_max - rrs.range_min)) + rrs.range_min
			s = strings.Join(split, fmt.Sprintf("%d", rand))
		}
	}
	rrs.current_index++
	return s
}

func (rrs *RoundRobin) StringAt(i int) string { return rrs.strings[i] }

func NewRoundRobin(s []string, randomize bool, range_min int, range_max int) *RoundRobin {
	r := new(RoundRobin)
	r.strings = s
	r.length = len(s)
	r.current_index = 0

	r.randomize = randomize
	r.range_min = range_min
	r.range_max = range_max

	r.is_random = make([]bool, r.length)
	for i := 0; i < r.length; i++ {
		r.is_random[i] = strings.Count(r.StringAt(i), "%") > 0
	}

	return r
}
func sendMsg(a *net.TCPAddr, nb_msgs int, time_chan chan int64, nbmails_chan chan int, single bool, tos_str []string, froms_str []string, mails_str []string, auth string, body string, dont_stop bool, ipsrcs_str []string, hello string, quiet bool) {

	var err_str string
	var code int
	var end time.Time
	var err error
	var s *net.TCPConn
	var mails *RoundRobin
	var ips *RoundRobin
	var local *net.TCPAddr
	const max_int = int(^uint(0) >> 1)
	tos := NewRoundRobin(tos_str, true, 0, max_int)
	froms := NewRoundRobin(froms_str, true, 0, max_int)
	reconnect := false

	if mails_str != nil {
		mails = NewRoundRobin(mails_str, false, 0, 0)
	} else {
		mails = nil
	}

	if ipsrcs_str != nil {
		ips = NewRoundRobin(ipsrcs_str, true, 1, 254)
	} else {
		ips = nil
	}
	begin := time.Now()

	if ips != nil {
		ip := ips.Peek()
		local, err = net.ResolveTCPAddr("tcp4", ip + ":0")
		/* ignore error, and bind to the default IP */
		if err != nil {
			log.Println("Cannot resolve ip address: " + ip)
		}
	}

	if single {
		s, err = connect_s(local, a, hello)
		if err != nil {
			goto err_label
		}
	}

	for i := 0; dont_stop || i < nb_msgs; i++ {
		if !single || reconnect {
			if ips != nil {
				ip := ips.Peek()
				local, err = net.ResolveTCPAddr("tcp4", ip + ":0")
				/* ignore error, and bind to the default IP */
				if err != nil {
					log.Println("Cannot resolve ip address: " + ip)
				}
			}
			s, err = connect_s(local, a, hello)
			if err != nil {
				goto err_label
			}
		}

		/*
		 * AUTH:
		 */
		if auth != "" {
			data := make([]byte, 1024)
			in := make([]uint8, len(auth))

			strings.NewReader(auth).Read(in)
			base64.StdEncoding.Encode(data, in)

			msg := fmt.Sprintf("AUTH PLAIN %s\r\n", string(data))
			code, err, err_str = write(s, msg)
			if verbose {
				log.Println(msg)
			}
			if err != nil {
				goto err_label
			}
			if code > 399 {
				log.Println(err_str)
				reconnect = true
				continue
			}
		}

		/*
		 * MAIL FROM:
		 */
		from := froms.Peek()

		msg := fmt.Sprintf("MAIL FROM:%s\r\n", from)
		code, err, err_str = write(s, msg)
		if verbose {
			log.Println(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Println(err_str)
			reconnect = true
			continue
		}

		/*
		 * RCPT TO:
		 */
		rcpt_tos := strings.Split(tos.Peek(), ",")
		for j := 0; j < len(rcpt_tos); j++ {
			msg = fmt.Sprintf("RCPT TO:%s\r\n", rcpt_tos[j])
			code, err, err_str = write(s, msg)
			if verbose {
				log.Println(msg)
			}
			if err != nil {
				goto err_label
			}
			if code > 399 {
				log.Println(err_str)
				reconnect = true
				continue
			}

		}

		/*
		 * DATA:
		 */
		msg = "DATA\r\n"
		code, err, err_str = write(s, msg)
		if verbose {
			log.Println(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Println(err_str)
			reconnect = true
			continue
		}


		/*
		 * MSG
		 */
		if mails == nil {
			if body != "" {
				msg = body
			} else {
				msg = "blah"
			}
		} else {
			msg = mails.Peek()
		}

		msg += "\r\n.\r\n"
		code, err, err_str = write(s, msg)
		if verbose {
			log.Println(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Println(err_str)
			reconnect = true
			continue
		}
		if (!quiet) {
			nbmails_chan <- 1
		}
		if !single {
			err = close_s(s)
			if err != nil {
				goto err_label
			}
		}
		if reconnect {
			reconnect = false
		}
	}
	if single {
		err = close_s(s)
		if err != nil {
			goto err_label
		}
	}

	end = time.Now()
	time_chan <- int64(end.Sub(begin)) / 1000 / int64(nb_msgs)
	return

err_label:
	log.Println(err)
	time_chan <- 1
	return
}

func abs(i int) int {
	if i >= 0 {
		return i
	} else {
		return -i
	}
	return i
}
func showProgress(nbmails_chan chan int, total_mails int) {
	current_mails := 0
	percent := 0
	length := 22
	last_pct := -1
	first := true

	for {
		current_mails += <-nbmails_chan
		percent = current_mails * 100 / total_mails

		if percent*length/100 == last_pct {
			continue
		}
		last_pct = percent * length / 100

		if !first {
			for i := 0; i < length+2; i++ {
				fmt.Printf("%c", 8)
			}
		} else {
			first = false
		}
		fmt.Printf("[")
		for i := 0; i < percent*length/100; i++ {
			fmt.Printf("=")
		}
		for i := 0; i < (100-percent)*length/100; i++ {
			fmt.Printf(" ")
		}
		fmt.Printf("]")
		diff := (percent * length / 100) + ((100 - percent) * length / 100)
		for i := 0; i < length-abs(diff); i++ {
			fmt.Printf(" ")
		}
	}
	//log.Println(empty);
}

func main() {
	var quiet bool
	time_chan := make(chan int64)
	nbmails_chan := make(chan int, 128)
	var port, nb_threads, nb_msgs, msg_size int
	var auth, body, host, from, to, maildir, ipsrc, hello string
	var single, dont_stop bool
	var msgs []string
	var ipsrcs []string

	flag.IntVar(&port, "port", 25, "TCP port")
	flag.IntVar(&nb_threads, "nb_threads", 10, "Number of concurrent threads")
	flag.IntVar(&nb_msgs, "nb_msgs", 500, "Number of messages")
	flag.IntVar(&msg_size, "msg_size", 6, "Message size in bytes, overrides -body")
	flag.BoolVar(&single, "single", false, "Open only one session per thread")
	flag.BoolVar(&dont_stop, "dont-stop", false, "Never stop sending email (ignores -nb_msgs)")
	flag.StringVar(&host, "host", "127.0.0.1", "smtp host")
	flag.StringVar(&hello, "hello", "localhost", "hello string")
	flag.StringVar(&from, "from", "from@example.org", "mail from (separated by ':')\n\t\t'%' is replaced by random number")
	flag.StringVar(&to, "to", "to@example.org", "mail from (separated by ':')\n\t\t'%' replaced by random number")
	flag.StringVar(&ipsrc, "ipsrc", "", "Originating ip source (separated by ':')\n\t\t'%' replaced by random number [1-254]")
	flag.StringVar(&maildir, "maildir", "", "Load emails to send from maildir")
	flag.StringVar(&auth, "auth", "", "Authentication password (AUTH PLAIN)")
	flag.StringVar(&body, "body", "blah", "Body of the message")
	flag.BoolVar(&verbose, "verbose", false, "Display client/server communications")
	flag.BoolVar(&quiet, "quiet", false, "Don't display the progress bar")

	flag.Parse()

	if maildir != "" {
		files, err := ioutil.ReadDir(maildir)
		if err != nil {
			log.Panic(err)
		}

		num_files := 0
		for i := 0; i < len(files); i++ {
			if !files[i].IsDir() {
				num_files++
			}
		}

		msgs = make([]string, num_files)
		for i := 0; i < len(files); i++ {
			if !files[i].IsDir() {
				filename := fmt.Sprintf("%s/%s", maildir , files[i].Name())
				b, err := ioutil.ReadFile(filename)
				if err != nil {
					log.Println("Cannot read filename " + filename)
					continue
				}

				msgs[i] = string(b)
			}
		}
	}

	if msg_size > 0 {
		body = strings.Repeat("a", msg_size);
	}

	tos := strings.Split(to, ":")
	if tos == nil {
		tos = make([]string, 1);
		tos[0] = to;
	}

	froms := strings.Split(from, ":")
	if froms == nil {
		froms = make([]string, 1);
		froms[0] = from;
	}

	a, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Panic(err)
	}

	if ipsrc != "" {
		ipsrcs = strings.Split(ipsrc, ":")
	} else {
		ipsrcs = nil
	}
	if !quiet {
		go showProgress(nbmails_chan, nb_threads*nb_msgs)
	}
	for i := 0; i < nb_threads; i++ {
		go sendMsg(a, nb_msgs, time_chan,
			nbmails_chan, single, tos,
			froms, msgs, auth, body, dont_stop, ipsrcs, hello, quiet)
	}

	var avg_time int64 = 0
	for t := 0; t < nb_threads; t++ {
		avg_time += <-time_chan
	}

	fmt.Printf("\nAverage processing time: %d\n", avg_time/int64(nb_threads))
	return
}
