package main

import (
    "fmt"
    "net/http"
    "os"
    "log"
    "image/png"
    "time"
    "encoding/gob"
    "github.com/boombuler/barcode"
    "github.com/boombuler/barcode/qr"
    "github.com/Rhymen/go-whatsapp"
)

var wac, err = whatsapp.NewConn(5 * time.Second)

type waHandler struct {
    c *whatsapp.Conn
}

func hello(w http.ResponseWriter, req *http.Request) {
    qrbody := make(chan string)
    var error bool = false
    var qrb string
    if err != nil {
        error = true
        qrbody <- fmt.Sprintf("error creating connection: %v\n", err)
    }else{
        go func() {
            err := login(wac, qrbody);
            if err != nil {
                error = true
                qrbody <- fmt.Sprintf("error logging in: %v\n", err)
            }
        }()
    }
    qrb = <-qrbody
    if !error {
        if qrb == "resok"{
            pong, err := wac.AdminTest()

            if !pong || err != nil {
                log.Fatalf("error pinging in: %v\n", err)
            }
            fmt.Fprintf(w, "Connected to whatsapp")
            wac.AddHandler(&waHandler{wac})
        }else{
            qrCode, _ := qr.Encode(qrb, qr.L, qr.Auto)
            qrCode, _ = barcode.Scale(qrCode, 512, 512)
            png.Encode(w, qrCode)
        }
    }else{
        fmt.Fprintf(w, qrb)
    }
    // png.Encode(w, login)

    //verifies phone connectivity
    // pong, err := wac.AdminTest()

    // if !pong || err != nil {
    //     log.Fatalf("error pinging in: %v\n", err)
    // }
    // fmt.Println("Pinging ...")
    // wac.AddHandler(&waHandler{wac})
    // sinput()
}

func headers(w http.ResponseWriter, req *http.Request) {

    for name, headers := range req.Header {
        for _, h := range headers {
            fmt.Fprintf(w, "%v: %v\n", name, h)
        }
    }
}

func main() {
    port := os.Getenv("PORT")

    if port == "" {
        log.Fatal("$PORT must be set")
    }
    // var port = "8090" //

    http.HandleFunc("/hello", hello)
    http.HandleFunc("/headers", headers)

    http.ListenAndServe(":"+port, nil)
}

//HandleError needs to be implemented to be a valid WhatsApp handler
func (h *waHandler) HandleError(err error) {

    if e, ok := err.(*whatsapp.ErrConnectionFailed); ok {
        log.Printf("Connection failed, underlying error: %v", e.Err)
        log.Println("Waiting 30sec...")
        <-time.After(30 * time.Second)
        log.Println("Reconnecting...")
        err := h.c.Restore()
        if err != nil {
            log.Fatalf("Restore failed: %v", err)
        }
    } else {
        log.Printf("error occoured: %v\n", err)
    }
}

//Optional to be implemented. Implement HandleXXXMessage for the types you need.
func (*waHandler) HandleTextMessage(message whatsapp.TextMessage) {
    fmt.Printf("%v %v %v %v\n\t%v\n", message.Info.Timestamp, message.Info.Id, message.Info.RemoteJid, message.ContextInfo.QuotedMessageID, message.Text)
    msg := whatsapp.TextMessage{
        Info: whatsapp.MessageInfo{
            RemoteJid: message.Info.RemoteJid,
        },
        Text:message.Text,
    }
    msgId, err := wac.Send(msg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error sending message: %v", err)
    } else {
        fmt.Println("Message Sent -> ID : " + msgId)
    }
}

// func sinput(){
//     buf := bufio.NewReader(os.Stdin)
//     fmt.Print("> ")
//     sentence, err := buf.ReadBytes('\n')
//     if err != nil {
//         fmt.Println(err)
//     } else {
//         var raw []string = strings.SplitAfterN(string(sentence), " ", 3)
//         var cmd = strings.ReplaceAll(raw[0], " ", "")
//         switch(cmd){
//             case "send":
//                 msg := whatsapp.TextMessage{
//                     Info: whatsapp.MessageInfo{
//                         RemoteJid: strings.ReplaceAll(raw[1], " ", "") + "@s.whatsapp.net",
//                     },
//                     Text:raw[2],
//                 }
//                 msgId, err := wac.Send(msg)
//                 if err != nil {
//                     fmt.Fprintf(os.Stderr, "error sending message: %v", err)
//                 } else {
//                     fmt.Println("Message Sent -> ID : " + msgId)
//                 }
//                 break;
//             case "close":
//                 //Disconnect safe
//                 fmt.Println("Shutting down now.")
//                 session, err := wac.Disconnect()
//                 if err != nil {
//                     log.Fatalf("error disconnecting: %v\n", err)
//                 }
//                 if err := writeSession(session); err != nil {
//                     log.Fatalf("error saving session: %v", err)
//                 }
//                 break;
//         }
//     }

//     // strings.ReplaceAll(
//     sinput()
// }

func login(wac *whatsapp.Conn, qrbody chan<- string) error {
    //load saved session
    session, err := readSession()
    var qrCode string
    if err == nil{
        //restore session
        session, err = wac.RestoreWithSession(session)
        if err != nil {
            return fmt.Errorf("restoring failed: %v\n", err)
        }else{
            qrbody <- fmt.Sprintf("resok")
        }
    } else {
        //no saved session -> regular login
        qr2 := make(chan string)
        go func() {
            qrCode = <-qr2
            log.Printf(qrCode)
            qrbody <- fmt.Sprintf(qrCode)
        }()
        session, err = wac.Login(qr2)
        if err != nil {
            return fmt.Errorf("error during login: %v\n", err)
        }else{
            wac.AddHandler(&waHandler{wac})
        }
    }
    err = writeSession(session)
    if err != nil {
        return fmt.Errorf("error saving session: %v\n", err)
    }
    return nil
}

func readSession() (whatsapp.Session, error) {
    session := whatsapp.Session{}
    file, err := os.Open("whatsappSession.gob")
    if err != nil {
        return session, err
    }
    defer file.Close()
    decoder := gob.NewDecoder(file)
    err = decoder.Decode(&session)
    if err != nil {
        return session, err
    }
    return session, nil
}

func writeSession(session whatsapp.Session) error {
    file, err := os.Create("whatsappSession.gob")
    if err != nil {
        return err
    }
    defer file.Close()
    encoder := gob.NewEncoder(file)
    err = encoder.Encode(session)
    if err != nil {
        return err
    }
    return nil
}