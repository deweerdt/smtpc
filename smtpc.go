package main

import (
	"net"
	"log"
	"os"
	"fmt"
	"io/ioutil"
	"flag"
	"rand"
	"time"
	"strings"
	"strconv"
)

func write(s *net.TCPConn, str string) (code int, err os.Error, err_str string) {
	code = 200

	_, err = s.Write(strings.Bytes(str))
	if err != nil {
		return
	}

	var b [1024]byte
	_, err = s.Read(&b)
	if err != nil {
		return
	}

	err_str = string(b[0:])
	code, err = strconv.Atoi(err_str[0:3]);

	return
}

var verbose bool

func close_s(s *net.TCPConn) (err os.Error) {
	var err_str string
	var code int

	code, err, err_str = write(s, "QUIT\r\n")
	if verbose {
		log.Stderr("QUIT\r\n")
	}
	if err != nil {
		return
	}
	if code > 399 {
		log.Stderr(err_str)
	}

	s.Close()
	return
}

func connect_s(a *net.TCPAddr) (s *net.TCPConn, err os.Error) {
	var err_str string
	var code int

	s, err = net.DialTCP("tcp", nil, a)
	if err != nil {
		return
	}

	_, err, err_str = write(s, "EHLO localhost\r\n")
	if verbose {
		log.Stderr("EHLO localhost\r\n")
	}
	if err != nil {
		return
	}
	if code > 399 {
		log.Stderr(err_str)
	}

	return
}

type RoundRobin struct {
	strings       []string
	length        int
	current_index int
	randomize     bool
	is_random     []bool
}

func (rrs *RoundRobin) Peek() string {
	s := rrs.strings[rrs.current_index % rrs.length]
	if rrs.randomize {
		if rrs.is_random[rrs.current_index % rrs.length] {
			split := strings.Split(s, "%", 0)
			s = strings.Join(split, fmt.Sprintf("%d", rand.Int()))
		}
	}
	rrs.current_index++
	return s
}

func (rrs *RoundRobin) StringAt(i int) string { return rrs.strings[i] }

func NewRoundRobin(s []string, randomize bool) *RoundRobin {
	r := new(RoundRobin)
	r.strings = s
	r.length = len(s)
	r.current_index = 0
	r.randomize = randomize
	r.is_random = make([]bool, r.length)
	for i := 0; i < r.length; i++ {
		r.is_random[i] = strings.Count(r.StringAt(i), "%") > 0
	}

	return r
}
func sendMsg(a *net.TCPAddr, nb_msgs int, time_chan chan int64, nbmails_chan chan int, single bool, tos_str []string, froms_str []string, mails_str []string) {

	var err_str string
	var code int
	var err os.Error
	var s *net.TCPConn
	var mails *RoundRobin
	tos := NewRoundRobin(tos_str, true)
	froms := NewRoundRobin(froms_str, true)
	if mails_str != nil {
		mails = NewRoundRobin(mails_str, false)
	} else {
		mails = nil
	}

	begin := time.Nanoseconds()

	if single {
		s, err = connect_s(a)
		if err != nil {
			goto err_label
		}
	}

	for i := 0; i < nb_msgs; i++ {
		if !single {
			s, err = connect_s(a)
			if err != nil {
				goto err_label
			}
		}

		/*
		 * MAIL FROM:
		 */
		from := froms.Peek()

		msg := fmt.Sprintf("MAIL FROM:%s\r\n", from);
		code, err, err_str = write(s, msg)
		if verbose {
			log.Stderr(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Stderr(err_str)
		}

		/*
		 * RCPT TO:
		 */
		rcpt_tos := strings.Split(tos.Peek(), ",", 0)
		for j := 0; j < len(rcpt_tos); j++ {
			msg = fmt.Sprintf("RCPT TO:%s\r\n", rcpt_tos[j]);
			code, err, err_str = write(s, msg)
			if verbose {
				log.Stderr(msg)
			}
			if err != nil {
				goto err_label
			}
			if code > 399 {
				log.Stderr(err_str)
			}

		}

		/*
		 * DATA:
		 */
		msg = "DATA\r\n"
		code, err, err_str = write(s, msg)
		if verbose {
			log.Stderr(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Stderr(err_str)
		}


		/*
		 * MSG
		 */
		if mails == nil {
			msg = "blah"
		} else {
			msg = mails.Peek()
		}

		msg += "\r\n.\r\n"
		code, err, err_str = write(s, msg)
		if verbose {
			log.Stderr(msg)
		}
		if err != nil {
			goto err_label
		}
		if code > 399 {
			log.Stderr(err_str)
		}


		nbmails_chan <- 1
		if !single {
			err = close_s(s)
			if err != nil {
				goto err_label
			}
		}
	}
	if single {
		err = close_s(s)
		if err != nil {
			goto err_label
		}
	}

	end := time.Nanoseconds()
	time_chan <- ((end - begin) / 1000 / int64(nb_msgs))
	return

err_label:
	log.Exit(err)
	time_chan <- 1
	return
}

func abs(i int) (int) {
	if i >= 0 {
		return i;
	} else {
		return -i;
	}
	return i;
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

		if percent * length / 100 == last_pct {
			continue
		}
		last_pct = percent * length / 100

		if !first {
			for i := 0; i < length + 2; i++ {
				fmt.Printf("%c", 8);
			}
		} else {
			first = false
		}
		fmt.Printf("[")
		for i := 0; i < percent * length / 100; i++ {
			fmt.Printf("=");
		}
		for i := 0; i < (100 - percent) * length / 100; i++ {
			fmt.Printf(" ");
		}
		fmt.Printf("]")
		diff := (percent * length / 100) + ((100 - percent) * length / 100)
		for i := 0; i < length - abs(diff); i++ {
			fmt.Printf(" ");
		}
	}
	//log.Stderr(empty);
}

func main() {
	var quiet bool
	time_chan := make(chan int64)
	nbmails_chan := make(chan int)
	var port, nb_threads, nb_msgs int
	var host, from, to, maildir string
	var single bool
	var msgs []string

	flag.IntVar(&port, "port", 25, "TCP port")
	flag.IntVar(&nb_threads, "nb_threads", 10, "Number of concurrent threads")
	flag.IntVar(&nb_msgs, "nb_msgs", 500, "Number of messages")
	flag.BoolVar(&single, "single", false, "Open only one session per thread")
	flag.StringVar(&host, "host", "127.0.0.1", "smtp host")
	flag.StringVar(&from, "from", "from@example.org", "mail from (separated by ':')\n\t\t'%' is replaced by random number")
	flag.StringVar(&to, "to", "to@example.org", "mail from (separated by ':')\n\t\t'%' replaced by random number")
	flag.StringVar(&maildir, "maildir", "", "Load emails to send from maildir")
	flag.BoolVar(&verbose, "verbose", false, "Display client/server communications")
	flag.BoolVar(&quiet, "quiet", false, "Don't display the progress bar")

	flag.Parse()

	if maildir != "" {
		files, err := ioutil.ReadDir(maildir)
		if err != nil {
			log.Exit(err)
		}

		num_files := 0
		for i := 0; i < len(files); i++ {
			if files[i].IsRegular() {
				num_files++
			}
		}

		msgs = make([]string, num_files)
		for i := 0; i < len(files); i++ {
			if files[i].IsRegular() {
				filename := maildir + "/" + files[i].Name
				b, err := ioutil.ReadFile(filename)
				if err != nil {
					log.Stderr("Cannot read filename " + filename)
					continue
				}

				msgs[i] = string(b)
			}
		}
	}

	tos := strings.Split(to, ":", 0)
	froms := strings.Split(from, ":", 0)

	a, err := net.ResolveTCPAddr(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Exit(err)
	}

	for i := 0; i < nb_threads; i++ {
		go sendMsg(a, nb_msgs, time_chan, nbmails_chan, single, tos, froms, msgs)
	}

	if (!quiet) {
		go showProgress(nbmails_chan, nb_threads * nb_msgs)
	}
	var avg_time int64 = 0
	for t := 0; t < nb_threads; t++ {
		avg_time += <-time_chan
	}

	fmt.Printf("\nAverage processing time: %d\n", avg_time/int64(nb_threads))
	return
}
