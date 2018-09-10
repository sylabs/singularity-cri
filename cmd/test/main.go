package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func main() {
	cc, err := grpc.Dial(os.Args[1], grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	c := k8s.NewImageServiceClient(cc)
	defer cc.Close()

	log.Println("Listening to your commands...")
	defer log.Println("Stopped listening.")

	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		args := strings.Split(scan.Text(), " ")
		switch cmd := args[0]; cmd {
		case "pull":
			resp, err := c.PullImage(context.Background(), &k8s.PullImageRequest{
				Image: &k8s.ImageSpec{
					Image: args[1],
				},
			})
			if err != nil {
				log.Printf("Could not pull image: %v", err)
				continue
			}
			fmt.Printf("\tImage reference: %s\n", resp.ImageRef)
		case "list":
			resp, err := c.ListImages(context.Background(), &k8s.ListImagesRequest{
				Filter: &k8s.ImageFilter{
					Image: &k8s.ImageSpec{},
				},
			})
			if err != nil {
				log.Printf("Could not list images: %v", err)
				continue
			}
			for _, image := range resp.Images {
				fmt.Printf("\tImage: %s\n\tSize: %d\n\tTags: %v\n\tDigests: %v\n",
					image.Id, image.Size_, image.RepoTags, image.RepoDigests)
			}
		case "remove":
			_, err := c.RemoveImage(context.Background(), &k8s.RemoveImageRequest{
				Image: &k8s.ImageSpec{
					Image: args[1],
				},
			})
			if err != nil {
				log.Printf("Could not remove image: %v", err)
				continue
			}
			fmt.Printf("\tImage removed\n")
		case "stat":
			resp, err := c.ImageStatus(context.Background(), &k8s.ImageStatusRequest{
				Image: &k8s.ImageSpec{
					Image: args[1],
				},
			})
			if err != nil {
				log.Printf("Could not query image stat: %v", err)
				continue
			}
			if resp.Image == nil {
				fmt.Printf("\tImage not found\n")
				continue
			}
			fmt.Printf("\tImage: %s\n\tSize: %d\n\tTags: %v\n\tDigests: %v\n",
				resp.Image.Id, resp.Image.Size_, resp.Image.RepoTags, resp.Image.RepoDigests)
		case "exit":
			return
		default:
			fmt.Printf("Unknown command %q\n", cmd)
		}
	}
}
