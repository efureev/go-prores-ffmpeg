package prores

import "fmt"
import "github.com/amarburg/goav/avcodec"
import "github.com/amarburg/goav/avutil"
import "github.com/amarburg/goav/swscale"

import "image"

// import "encoding/binary"
// import "bytes"

import "C"
import "unsafe"

import "errors"

// Global, boo
var prores *avcodec.Codec

func init() {
	avcodec.AvcodecRegisterAll()

	prores = avcodec.AvcodecFindDecoder(avcodec.CodecId(avcodec.AV_CODEC_ID_PRORES))
	if prores == nil {
		panic("Couldn't find ProRes codec")
	}

	if prores.AvCodecIsDecoder() != 1 {
		panic("Isn't a decoder")
	}

}

// DecodeProRes takes a byte slice containing a single ProRes frame, and uses goav (w/ ffmpeg)
// to produce a Go RGBA image.   This function requires that the frame width and height be known
// in advance.   It returns a pointer to the new image if successful, or nil and an error if
// unsuccessful.
//
// Note this function is still pretty rough.
func DecodeProRes(buf []byte, width int, height int) (*image.NRGBA, error) {

	if prores == nil {
		return nil, fmt.Errorf("Couldn't initialize ProRes")
	}

	ctx := prores.AvcodecAllocContext3()
	if ctx == nil {
		return nil, errors.New("Couldn't allocate context")
	}
	defer avcodec.AvcodecFreeContext(ctx)

	res := ctx.AvcodecOpen2(prores, nil)
	if res < 0 {
		return nil, errors.New(fmt.Sprintf("Couldn't open context (%d)", res))
	}

	packet := avcodec.AvPacketAlloc()
	packet.AvInitPacket()
	defer avcodec.AvPacketFree(packet)

	//if packet == nil { panic("Couldn't allocate packet") }

	//fmt.Printf("%v\n", packet)

	// Force a copy of the byte buffer ... why?  Because libav takes ownership and
	// frees it (eventually)
	packet.AvPacketFromByteSlice(buf)

	//if res < 0 { panic(fmt.Sprintf("Couldn't set avpacket data (%d)",res))}

	// And a frame to receive the data
	videoFrame := avutil.AvFrameAlloc()
	if videoFrame == nil {
		return nil, errors.New("Couldn't allocate destination frame")
	}
	//defer avutil.AvFrameUnref( videoFrame )
	defer avutil.AvFrameFree(videoFrame) // why does this segfault?

	ctx.SetWidth(width)
	ctx.SetHeight(height)

	// TODO.  Error handling
	res = ctx.SendPacket(packet)
	if res != 0 {
		return nil, errors.New(fmt.Sprintf("Error sending packet to decoder, err = %04x", -res))
	}

	// TODO.   May receive multiple frames from a packet, need to loop?
	res = ctx.ReceiveFrame(videoFrame)
	if res != 0 {
		return nil, errors.New(fmt.Sprintf("Error receiving frame from decoder, err = %04x", -res))
	}

	//fmt.Printf("Image is %d x %d, format %d\n", width, height, int(ctx.Pix_fmt) )

	// Convert frame to RGBA
	outputFmt := avcodec.AV_PIX_FMT_RGBA
	//dest_fmt := int32(avcodec.AV_PIX_FMT_RGBA)
	flags := 0
	ctxtSws := swscale.SwsGetcontext(width, height, swscale.PixelFormat(ctx.PixFmt()),
		width, height, swscale.PixelFormat(outputFmt),
		flags, nil, nil, nil)
	if ctxtSws == nil {
		return nil, errors.New("Couldn't open swscale context")
	}

	videoFrameRgb := avutil.AvFrameAlloc()
	if videoFrameRgb == nil {
		return nil, errors.New("Couldn't allocate destination frame")
	}
	defer avutil.AvFrameFree(videoFrameRgb) // why does this segfault?

	videoFrameRgb.SetWidth(ctx.Width())
	videoFrameRgb.SetHeight(ctx.Height())
	videoFrameRgb.SetFormat(avutil.PixelFormat(outputFmt))

	if res := avutil.AvFrameGetBuffer(videoFrameRgb, 8); res != 0 {
		return nil, fmt.Errorf("Error getting buffer %d", res)
	}

	swscale.SwsScale(ctxtSws, videoFrame.Data,
		videoFrame.Linesize,
		0, height,
		videoFrameRgb.Data,
		videoFrameRgb.Linesize)
	defer swscale.SwsFreecontext(ctxtSws)

	//fmt.Printf("%#v\n",*videoFrameRgb)


	//pixels := make([]byte, videoFrameRgb.Width * videoFrameRgb.Height * 4 )



	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	// This is incredibly slow
	//	reader := bytes.NewReader(rgb_data)
	//err := binary.Read(reader, binary.LittleEndian, &img.Pix)

	// nb. C.GoBytes makes a copy of the data
	// TODO: Endianness?
	img.Pix = C.GoBytes(unsafe.Pointer(videoFrameRgb.Data[0]),
		                    C.int(videoFrameRgb.Width()*videoFrameRgb.Height()*4))


	// if err != nil {
	// 	return nil, fmt.Errorf("error on binary read: %s", err.Error())
	// }

	return img, nil
}
