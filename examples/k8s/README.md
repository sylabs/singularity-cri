# Examples

## Hello Kubernetes 

This is a dummy HTTP server suitable to verify k8s installation.

```bash
kubectl apply -f hello-kubernetes.yaml
```

After that you should be able to hit [hello-kubernetes service](https://kubernetes.io/docs/concepts/services-networking/service) 
and see a web page with "Hello, World!" greeting.

## Image service

This is an HTTP server that serves images of cats.
Detailed description can be found [here](https://www.sylabs.io/guides/cri/1.0/user-guide/basic_usage.html#hello-cats).

```bash
kubectl apply -f image-service.yaml
```

After that you should be able to hit [image-service service](https://kubernetes.io/docs/concepts/services-networking/service) 
(page `/cats/good`) and see a bunch of cats images.

## Bookshelf service

Service for storing and searching books. Consists of two parts: mongoDB for storage and an
app that provides book API.

```bash
kubectl apply -f mongo.yaml
kubectl apply -f bookshelf.yaml
```

After that you should be able to interact with [bookshelf service](https://kubernetes.io/docs/concepts/services-networking/service).
The API is the following:

- List books
GET `/books`

- Create new book
POST `/books`
```json
{
"title": "Les Misérables",
"author": "Victor Hugo",
"published_date": "1862",
"description": "Examining the nature of law and grace, the novel elaborates upon the history of France, the architecture and urban design of Paris, politics, moral philosophy, antimonarchism, justice, religion, and the types and nature of romantic and familial love."
}
```

- Update existing book
PUT `/books/<id>`
```json
{
"title": "Les Misérables",
"author": "Victor Hugo",
"published_date": "June 1862",
"description": "Examining the nature of law and grace, the novel elaborates upon the history of France, the architecture and urban design of Paris, politics, moral philosophy, antimonarchism, justice, religion, and the types and nature of romantic and familial love."
}
```

- Get existing book
GET `/books/<id>`

- Delete existing book
POST `/books/<id>:delete`

## Darkflow image recognition

Image recognition service. Consists of three parts: backend image recognition service, front service for image
download, angular web service for UI.
Detailed description can be found [here](https://www.sylabs.io/guides/cri/1.0/user-guide/basic_usage.html#image-recognition-using-nvidia-gpu).

```bash
kubectl apply -f darkflow.yaml
```

After that you should be able to see web UI when accessing [darkflow service](https://kubernetes.io/docs/concepts/services-networking/service).
