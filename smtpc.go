package main

import (
	"net";
	"log";
	"os";
	"fmt";
	"flag";
	"rand";
	"time";
	"strings";
)

func ignore_read_then_write(s *net.TCPConn, str string) (err os.Error) {
	var b [1024]byte;
	_, err = s.Read(&b);
	if err != nil {
		return err
	}

	_, err = s.Write(strings.Bytes(str));
	if err != nil {
		return err
	}
	//fmt.Print(string(b));
	return nil;
}

func close_s(s *net.TCPConn) (err os.Error) {
	_, err = s.Write(strings.Bytes("quit\r\n"));
	if err != nil {
		return
	}
	b := make([]byte, 1024);
	_, err = s.Read(b);
	if err != nil {
		return
	}
	//fmt.Print(string(b));

	s.Close();
	return;
}

func connect_s(a *net.TCPAddr) (s *net.TCPConn, err os.Error) {
	s, err = net.DialTCP("tcp", nil, a);
	if err != nil {
		return
	}

	_, err = s.Write(strings.Bytes("ehlo localhost\r\n"));
	if err != nil {
		return
	}

	var b [1024]byte;
	_, err = s.Read(&b);
	if err != nil {
		return
	}
	//fmt.Print(string(b[0:]));
	return;
}

func sendMsg(a *net.TCPAddr, nb_msgs int, c chan int64, single bool, tos []string, froms []string) {
	var err os.Error;
	var s *net.TCPConn;
	to_index := 0;
	from_index := 0;
	to_len := len(tos);
	from_len := len(froms);
	to_is_random := make([]int, to_len);
	from_is_random := make([]int, from_len);

	for i := 0; i < to_len; i++ {
		to_is_random[i] = strings.Count(tos[i], "%")
	}

	for i := 0; i < from_len; i++ {
		from_is_random[i] = strings.Count(froms[i], "%")
	}

	begin := time.Nanoseconds();

	if single {
		s, err = connect_s(a);
		if err != nil {
			goto err_label
		}
	}

	for i := 0; i < nb_msgs; i++ {
		if !single {
			s, err = connect_s(a);
			if err != nil {
				goto err_label
			}
		}

		from := froms[from_index % from_len];
		to := tos[to_index % to_len];
		if to_is_random[to_index % to_len] > 0 {
			split := strings.Split(to, "%", 0);
			to = strings.Join(split, fmt.Sprintf("%d", rand.Int()));
		}
		if from_is_random[from_index % from_len] > 0 {
			split := strings.Split(from, "%", 0);
			from = strings.Join(split, fmt.Sprintf("%d", rand.Int()));
		}
		msg := "mail from:"+from+"\r\n";
		_, err = s.Write(strings.Bytes(msg));
		if err != nil {
			goto err_label
		}

		b := make([]byte, 1024);
		_, err = s.Read(b);
		if err != nil {
			goto err_label
		}

		msg = "rcpt to:"+to+"\r\n";
		_, err = s.Write(strings.Bytes(msg));
		if err != nil {
			goto err_label
		}

		b = make([]byte, 1024);
		_, err = s.Read(b);
		if err != nil {
			goto err_label
		}

		msg = "data\r\n"
		_, err = s.Write(strings.Bytes(msg));
		if err != nil {
			goto err_label
		}

		b = make([]byte, 1024);
		_, err = s.Read(b);
		if err != nil {
			goto err_label
		}

		msg = "blah\r\n.\r\n";
		_, err = s.Write(strings.Bytes(msg));
		if err != nil {
			goto err_label
		}

		b = make([]byte, 1024);
		_, err = s.Read(b);
		if err != nil {
			goto err_label
		}

		if !single {
			err = close_s(s);
			if err != nil {
				goto err_label
			}
		}

		to_index++;
		from_index++;
	}
	if single {
		err = close_s(s);
		if err != nil {
			goto err_label
		}
	}

	end := time.Nanoseconds();
	c <- ((end - begin) / 1000 / int64(nb_msgs));
	return;

err_label:
	log.Exit(err);
	c <- 1;
	return;
}

func main() {
	c := make(chan int64);
	var port, nb_threads, nb_msgs int;
	var host, from, to string;
	var single bool;

	flag.IntVar(&port, "port", 25, "TCP port");
	flag.IntVar(&nb_threads, "nb_threads", 10, "Number of concurrent threads");
	flag.IntVar(&nb_msgs, "nb_msgs", 500, "Number of messages");
	flag.BoolVar(&single, "single", false, "Open only one session per thread");
	flag.StringVar(&host, "host", "127.0.0.1", "smtp host");
	flag.StringVar(&from, "from", "from@example.org", "mail from (separated by ':')\n\t% replaced by random number");
	flag.StringVar(&to, "to", "to@example.org", "mail from (separated by ':')\n\t% replaced by random number");

	flag.Parse();

	tos := strings.Split(to, ":", 0);
	froms := strings.Split(from, ":", 0);

	a, err := net.ResolveTCPAddr(fmt.Sprintf("%s:%d", host, port));
	if err != nil {
		log.Exit(err)
	}

	for i := 0; i < nb_threads; i++ {
		go sendMsg(a, nb_msgs, c, single, tos, froms)
	}

	var avg_time int64 = 0;
	for t := 0; t < nb_threads; t++ {
		avg_time += <-c
	}

	fmt.Printf("Average processing time: %d\n", avg_time/int64(nb_threads));
	return;
}
