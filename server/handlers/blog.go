package handlers

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/vaibhav/grpc_crud/server/entity"
	blog_proto "github.com/vaibhav/grpc_crud/server/protos/blog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/status"
)

// for DB -
var (
	mongoClient *mongo.Client
	blogDBC     *mongo.Collection
	mongoCtx    context.Context
)

type BlogServiceServer struct{}

func (srv *BlogServiceServer) CreateBlog(ctx context.Context, req *blog_proto.CreateBlogReq) (*blog_proto.CreateBlogRes, error) {
	// -------------------------- GETTING DATA FROM PROTOCOL BUFFER --------------------------
	// getting fields from protobuf req methods, by default as string
	blog := req.GetBlog()

	// -------------------------- CREATING ENTITY OBJECT AND SAVING ---------------------------
	// Now we have to convert this into a BlogItem type to convert into BSON
	data := entity.BlogItem{
		// ID:       primitive.NilObjectID,
		AuthorID: blog.GetAuthorId(),
		Title:    blog.GetTitle(),
		Content:  blog.GetContent(),
	}

	// ------------------------------------ ADDING TO DB -----------------------------------------
	// insert data into database - *InsertOneResult contains the oid
	result, err := blogDBC.InsertOne(mongoCtx, data)
	if err != nil {
		// internal error related to GRPC handling, passing status.Errorf code as our error
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal server error: %v", err))
	}

	// adding the oid to the blog
	oid := result.InsertedID.(primitive.ObjectID) // ?
	blog.Id = oid.Hex()

	res := &blog_proto.CreateBlogRes{Blog: blog}
	return res, nil
}

func (srv *BlogServiceServer) ReadBlog(ctx context.Context, req *blog_proto.ReadBlogReq) (*blog_proto.ReadBlogRes, error) {
	// -------------------------- GETTING DATA FROM PROTOCOL BUFFER --------------------------
	id := req.GetId() // as string

	// --------------------------- CROSS CHECKING and FINDING IF AVAILABLE FROM DB ----------------------------
	// converting id to ObjectId , so to be verified with db entries
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("Invalid input by user, can't convert ObjectID: %v", err))
	}

	// finding the blog, with "_id" = oid
	result := blogDBC.FindOne(ctx, bson.M{"_id": oid})

	// creating empty entity object, and decoding result
	blog := entity.BlogItem{}

	if err := result.Decode(&blog); err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not found with ObjectId = %s : %v", id, err))
	}

	// ------------------------------------------ RETURN ----------------------------------------
	res := &blog_proto.ReadBlogRes{
		Blog: &blog_proto.Blog{
			Id:       oid.Hex(),
			AuthorId: blog.AuthorID,
			Title:    blog.Title,
			Content:  blog.Content,
		},
	}
	return res, nil
}

func (srv *BlogServiceServer) UpdateBlog(ctx context.Context, req *blog_proto.UpdateBlogReq) (*blog_proto.UpdateBlogRes, error) {
	blog := req.GetBlog()

	// Convert the Id string to a MongoDB ObjectId
	oid, err := primitive.ObjectIDFromHex(blog.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("Could not convert the supplied blog id to a MongoDB ObjectId: %v", err))
	}

	// Convert the data to be updated into an unordered Bson document
	update := bson.M{
		"authord_id": blog.GetAuthorId(),
		"title":      blog.GetTitle(),
		"content":    blog.GetContent(),
	}

	// Convert the oid into an unordered bson document to search by id
	filter := bson.M{"_id": oid}

	// Result is the BSON encoded result
	// so to return the updated document instead of original we have to add options.
	result := blogDBC.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, options.FindOneAndUpdate().SetReturnDocument(1))

	// Decode result and write it to 'decoded'
	decoded := entity.BlogItem{}
	err = result.Decode(&decoded)
	if err != nil {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Could not find blog with supplied ID: %v", err),
		)
	}

	res := &blog_proto.UpdateBlogRes{
		Blog: &blog_proto.Blog{
			Id:       decoded.ID.Hex(),
			AuthorId: decoded.AuthorID,
			Title:    decoded.Title,
			Content:  decoded.Content,
		},
	}
	return res, nil
}

func (srv *BlogServiceServer) DeleteBlog(ctx context.Context, req *blog_proto.DeleteBlogReq) (*blog_proto.DeleteBlogRes, error) {
	id := req.GetId() // as string

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("Invalid input by user, can't convert ObjectID: %v", err))
	}

	// DeleteOne returns DeleteResult which is a struct containing the amount of deleted docs (in this case only 1 always)
	// return a boolean instead
	_, err = blogDBC.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not find/delete blog with id %s: %v", id, err))
	}

	res := &blog_proto.DeleteBlogRes{
		Success: true,
	}
	return res, nil
}

// streaming- server side streaming (blog)
func (srv *BlogServiceServer) ListBlogs(req *blog_proto.ListBlogsReq, stream blog_proto.BlogService_ListBlogsServer) error {
	// Initiate a BlogItem type to write decoded data to
	data := &entity.BlogItem{}

	// collection.Find returns a cursor for our (empty) query
	cursor, err := blogDBC.Find(context.Background(), bson.M{})
	if err != nil {
		return status.Errorf(codes.Internal, fmt.Sprintf("Unknown internal error: %v", err))
	}
	defer cursor.Close(context.Background()) // closing cursor, before returning

	// cursor.Next() returns a boolean, if false there are no more items and loop will break
	for cursor.Next(context.Background()) {
		// Decode the data at the current pointer and write it to data
		err := cursor.Decode(data)
		if err != nil {
			return status.Errorf(codes.Unavailable, fmt.Sprintf("Could not decode data: %v", err))
		}

		// If no error is found send blog over stream
		stream.Send(&blog_proto.ListBlogsRes{
			Blog: &blog_proto.Blog{
				Id:       data.ID.Hex(),
				AuthorId: data.AuthorID,
				Content:  data.Content,
				Title:    data.Title,
			},
		})
	}

	// Check if the cursor has any errors
	if err := cursor.Err(); err != nil {
		return status.Errorf(codes.Internal, fmt.Sprintf("Unkown cursor error: %v", err))
	}
	return nil
}
