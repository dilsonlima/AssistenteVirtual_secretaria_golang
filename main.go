package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	//"github.com/go-rod/rod/lib/proto"
)

type Tarefa struct {
	ID      int
	Nome    string
	Horario time.Time
	Contato string
	Status  string
}

func carregarTarefas() ([]Tarefa, error) {
	arquivo, err := os.Open("tarefas.csv")
	if err != nil {
		return nil, err
	}
	defer arquivo.Close()

	leitor := csv.NewReader(arquivo)
	dados, err := leitor.ReadAll()
	if err != nil {
		return nil, err
	}

	var tarefas []Tarefa
	for i, linha := range dados {
		if i == 0 {
			continue
		}
		id, _ := strconv.Atoi(linha[0])
		horario, _ := time.Parse(time.RFC3339, linha[2])
		tarefas = append(tarefas, Tarefa{
			ID:      id,
			Nome:    linha[1],
			Horario: horario,
			Contato: linha[3],
			Status:  linha[4],
		})
	}
	return tarefas, nil
}

func enviarMensagem(contato, mensagem string) {
	url, err := launcher.New().Headless(false).Launch()
	if err != nil {
		log.Println("Erro ao iniciar o launcher:", err)
		return
	}
	browser := rod.New().ControlURL(url).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	defer page.Close()

	if err := page.Navigate("https://web.whatsapp.com"); err != nil {
		log.Println("Erro ao navegar para WhatsApp Web:", err)
		return
	}
	if err := page.WaitLoad(); err != nil {
		log.Println("Erro ao aguardar o carregamento do WhatsApp Web:", err)
		return
	}
	fmt.Println("Aguardando login manual no WhatsApp Web...")
	time.Sleep(30 * time.Second)

	err = page.Navigate("https://web.whatsapp.com/send?phone=" + contato + "&text=" + mensagem)
	if err != nil {
		log.Println("Erro ao navegar para o contato:", err)
		return
	}
	time.Sleep(10 * time.Second)
	err = page.Keyboard.Press('\n')  // Isso simula a tecla Enter
	if err != nil {
		log.Println("Erro ao pressionar Enter:", err)
		return
	}
	fmt.Println("Mensagem enviada para:", contato)
}

func monitorarTarefas() {
	for {
		tarefas, err := carregarTarefas()
		if err != nil {
			log.Println("Erro ao carregar tarefas:", err)
			continue
		}
		now := time.Now()
		for i, tarefa := range tarefas {
			if tarefa.Status == "pendente" && now.After(tarefa.Horario) && now.Sub(tarefa.Horario) < 5*time.Minute {
				mensagem := fmt.Sprintf("Tarefa '%s' agendada para %s não foi iniciada.", tarefa.Nome, tarefa.Horario.Format("15:04"))
				enviarMensagem(tarefa.Contato, mensagem)
				tarefas[i].Status = "notificado"
				salvarTarefas(tarefas)
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func salvarTarefas(tarefas []Tarefa) {
	arquivo, err := os.Create("tarefas.csv")
	if err != nil {
		log.Println("Erro ao salvar tarefas:", err)
		return
	}
	defer arquivo.Close()

	escritor := csv.NewWriter(arquivo)
	escritor.Write([]string{"ID", "Nome", "Horario", "Contato", "Status"})
	for _, t := range tarefas {
		escritor.Write([]string{
			strconv.Itoa(t.ID),
			t.Nome,
			t.Horario.Format(time.RFC3339),
			t.Contato,
			t.Status,
		})
	}
	escritor.Flush()
}

type PainelData struct {
	Tarefas []Tarefa
}

func painelHTML(w http.ResponseWriter, r *http.Request) {
	tarefas, _ := carregarTarefas()
	data := PainelData{Tarefas: tarefas}
	tmpl := template.Must(template.New("painel").Parse(`
<!DOCTYPE html>
<html lang="pt-br">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Painel de Tarefas</title>
    <style>
        body { font-family: Arial, sans-serif; background-color: #f3f4f6; padding: 2rem; }
        .container { max-width: 800px; margin: auto; background: white; padding: 2rem; border-radius: 8px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        h2 { color: #2563eb; }
        table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
        th, td { border: 1px solid #ccc; padding: 0.5rem; text-align: left; }
        th { background-color: #e5e7eb; }
        form input, form button { width: 100%; padding: 0.75rem; margin: 0.5rem 0; border-radius: 4px; border: 1px solid #ccc; }
        form button { background-color: #2563eb; color: white; border: none; cursor: pointer; }
        form button:hover { background-color: #1e40af; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Nova Tarefa</h2>
        <form method="POST" action="/nova">
            <input type="text" name="nome" placeholder="Nome da tarefa" required>
            <input type="datetime-local" name="horario" required>
            <input type="text" name="contato" placeholder="Número WhatsApp (+55...)" required>
            <button type="submit">Cadastrar</button>
        </form>
        <h2>Minhas Tarefas</h2>
        <table>
            <tr><th>ID</th><th>Nome</th><th>Horário</th><th>Contato</th><th>Status</th></tr>
            {{range .Tarefas}}
            <tr>
                <td>{{.ID}}</td>
                <td>{{.Nome}}</td>
                <td>{{.Horario.Format "02/01/2006 15:04"}}</td>
                <td>{{.Contato}}</td>
                <td>{{.Status}}</td>
            </tr>
            {{end}}
        </table>
    </div>
</body>
</html>
`))
	tmpl.Execute(w, data)
}

func novaTarefa(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	nome := r.FormValue("nome")
	horarioStr := r.FormValue("horario")
	contato := r.FormValue("contato")

	horario, _ := time.Parse("2006-01-02T15:04", horarioStr)
	id := int(time.Now().Unix())
	tarefa := Tarefa{
		ID:      id,
		Nome:    nome,
		Horario: horario,
		Contato: contato,
		Status:  "pendente",
	}
	tarefas, _ := carregarTarefas()
	tarefas = append(tarefas, tarefa)
	salvarTarefas(tarefas)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func main() {
	go monitorarTarefas()
	http.HandleFunc("/", painelHTML)
	http.HandleFunc("/nova", novaTarefa)
	fmt.Println("Servidor rodando em http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}