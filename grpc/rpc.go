package jsonrpc

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	builder "github.com/arduino/arduino-builder"
	"github.com/arduino/arduino-builder/types"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/arduino/arduino-builder/grpc/proto"
)

type StreamLogger struct {
	stream pb.Builder_BuildServer
}

func (s StreamLogger) Fprintln(w io.Writer, level string, format string, a ...interface{}) {
	s.stream.Send(&pb.Response{Line: fmt.Sprintf(format, a...)})
}

func (s StreamLogger) UnformattedFprintln(w io.Writer, str string) {
	s.stream.Send(&pb.Response{Line: str})
}

func (s StreamLogger) UnformattedWrite(w io.Writer, data []byte) {
	s.stream.Send(&pb.Response{Line: string(data)})
}

func (s StreamLogger) Println(level string, format string, a ...interface{}) {
	s.stream.Send(&pb.Response{Line: fmt.Sprintf(format, a...)})
}

func (s StreamLogger) Flush() string {
	return ""
}

func (s StreamLogger) Name() string {
	return "streamlogger"
}

type builderServer struct {
	resp    []*pb.Response
	ctx     *types.Context
	watcher *fsnotify.Watcher
}

// GetFeature returns the feature at the given point.
func (s *builderServer) Autocomplete(ctx context.Context, args *pb.BuildParams) (*pb.Response, error) {

	s.ctx.HardwareFolders = strings.Split(args.HardwareFolders, ",")
	s.ctx.ToolsFolders = strings.Split(args.ToolsFolders, ",")
	s.ctx.BuiltInLibrariesFolders = strings.Split(args.BuiltInLibrariesFolders, ",")
	s.ctx.OtherLibrariesFolders = strings.Split(args.OtherLibrariesFolders, ",")
	s.ctx.SketchLocation = args.SketchLocation
	s.ctx.CustomBuildProperties = strings.Split(args.CustomBuildProperties, ",")
	s.ctx.ArduinoAPIVersion = args.ArduinoAPIVersion
	s.ctx.FQBN = args.FQBN
	s.ctx.Verbose = false //p.Verbose
	s.ctx.BuildCachePath = args.BuildCachePath
	s.ctx.BuildPath = args.BuildPath
	s.ctx.WarningsLevel = args.WarningsLevel
	s.ctx.PrototypesSection = ""
	s.ctx.CodeCompleteAt = args.CodeCompleteAt

	s.ctx.IncludeFolders = s.ctx.IncludeFolders[0:0]
	s.ctx.LibrariesObjectFiles = s.ctx.LibrariesObjectFiles[0:0]
	s.ctx.CoreObjectsFiles = s.ctx.CoreObjectsFiles[0:0]
	s.ctx.SketchObjectFiles = s.ctx.SketchObjectFiles[0:0]

	s.ctx.ImportedLibraries = s.ctx.ImportedLibraries[0:0]

	err := builder.RunPreprocess(s.ctx)
	if err != nil {
		return &pb.Response{Line: s.ctx.GetLogger().Flush()}, err
	}

	// No feature was found, return an unnamed feature
	return &pb.Response{Line: s.ctx.GetLogger().Flush()}, nil
}

// GetFeature returns the feature at the given point.
func (s *builderServer) Build(args *pb.BuildParams, stream pb.Builder_BuildServer) error {

	s.ctx.HardwareFolders = strings.Split(args.HardwareFolders, ",")
	s.ctx.ToolsFolders = strings.Split(args.ToolsFolders, ",")
	s.ctx.BuiltInLibrariesFolders = strings.Split(args.BuiltInLibrariesFolders, ",")
	s.ctx.OtherLibrariesFolders = strings.Split(args.OtherLibrariesFolders, ",")
	s.ctx.SketchLocation = args.SketchLocation
	s.ctx.CustomBuildProperties = strings.Split(args.CustomBuildProperties, ",")
	s.ctx.ArduinoAPIVersion = args.ArduinoAPIVersion
	s.ctx.FQBN = args.FQBN
	s.ctx.Verbose = args.Verbose
	s.ctx.BuildCachePath = args.BuildCachePath
	s.ctx.BuildPath = args.BuildPath
	s.ctx.WarningsLevel = args.WarningsLevel
	s.ctx.PrototypesSection = ""
	s.ctx.CodeCompleteAt = ""

	s.ctx.IncludeFolders = s.ctx.IncludeFolders[0:0]
	s.ctx.LibrariesObjectFiles = s.ctx.LibrariesObjectFiles[0:0]
	s.ctx.CoreObjectsFiles = s.ctx.CoreObjectsFiles[0:0]
	s.ctx.SketchObjectFiles = s.ctx.SketchObjectFiles[0:0]

	s.ctx.ImportedLibraries = s.ctx.ImportedLibraries[0:0]

	// setup logger to send via protobuf
	oldlogger := s.ctx.GetLogger()
	logger := StreamLogger{stream}
	s.ctx.SetLogger(logger)

	err := builder.RunBuilder(s.ctx)
	s.ctx.SetLogger(oldlogger)
	if err != nil {
		return err
	}

	// No feature was found, return an unnamed feature
	return nil
}

/*
func (h *WatchHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p WatchParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	err := h.watcher.Add(p.Path)
	if err != nil {
		return nil, jsonrpc.ErrInvalidParams()
	}
	return BuildResult{
		Message: "OK " + p.Path,
	}, nil
}
*/

func startWatching(ctx *types.Context) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				ctx.CanUseCachedTools = false
				log.Println("event:", event)
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()
	return watcher
}

func newServer(ctx *types.Context, watcher *fsnotify.Watcher) *builderServer {
	s := new(builderServer)
	s.ctx = ctx
	s.watcher = watcher
	return s
}

func RegisterAndServeJsonRPC(ctx *types.Context) {

	watcher := startWatching(ctx)

	lis, _ := net.Listen("tcp", "localhost:12345")
	grpcServer := grpc.NewServer()
	pb.RegisterBuilderServer(grpcServer, newServer(ctx, watcher))
	grpcServer.Serve(lis)
}