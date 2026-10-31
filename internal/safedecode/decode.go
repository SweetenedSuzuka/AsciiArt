// safedecode 把未校验的图片文件路径转换成校验通过的图像数据。
// 体积很小的压缩文件在解码时可能占用极大的内存，因此在分配内存之前，每个输入都要先经过上限检查。
//
// 具体做法如下：
// - 只打开普通文件，不打开设备、管道和目录。
// - 拒绝超过 limits.MaxFileBytes 的文件。
// - 按文件头部字节判断格式，不依赖文件扩展名。
// - 在分配像素存储之前，先校验头部的宽和高。
// - 只支持 Go 标准库的三种解码器。
package safedecode

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"

	"asciiart/internal/limits"
)

// Format 标识一种受支持的图片封装格式。
type Format string

const (
	FormatPNG  Format = "PNG"
	FormatJPEG Format = "JPEG"
	FormatGIF  Format = "GIF"
)

// Image 以 Go 标准库的预乘 RGBA 布局保存解码后的像素。
// 采用预乘布局是出于设计目的：渲染器的第一步是把每张图像合成到不透明背景上，在预乘空间中这次合成只是一次加法，没有除法，没有取整误差，透明像素也不会带出错误的颜色。
type Image struct {
	Pix    []byte // RGBA 布局，逐像素 4 字节，透明度为预乘。
	Width  int
	Height int
	Format Format
}

// 当内容不是 PNG、JPEG 或 GIF 时，返回 ErrUnsupportedFormat。
var ErrUnsupportedFormat = errors.New("不支持的图片格式（仅支持 PNG / JPEG / GIF）")

// Decode 读取 path 指向的文件并返回其像素数据。
// Pix 的长度始终等于 Width*Height*4 字节，调用方不需要考虑行步长对齐。
func Decode(path string) (*Image, error) {
	f, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 按固定的字节上限读取整个文件。仅使用 LimitReader 会静默截断内容，解码器可能把被截断的前缀当作可以接受的残缺图像，因此这里另外检查文件长度。
	fileBytes, err := io.ReadAll(io.LimitReader(f, limits.MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取文件失败：%w", err)
	}
	if int64(len(fileBytes)) > limits.MaxFileBytes {
		return nil, fmt.Errorf("文件超过 %d MiB 上限", limits.MaxFileBytes>>20)
	}
	if len(fileBytes) == 0 {
		return nil, errors.New("文件为空")
	}

	format, err := detectFormat(fileBytes)
	if err != nil {
		return nil, err
	}

	config, err := decodeConfig(format, fileBytes)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 头部失败：%w", format, err)
	}
	// 头部校验发生在任何像素缓冲区分配之前，因此一个声明超大尺寸的小文件最多浪费几百字节的内存，不会被解码成几个 GB 的数据。
	if err := limits.CheckPixelCount(config.Width, config.Height); err != nil {
		return nil, err
	}

	src, err := decodeImage(format, fileBytes)
	if err != nil {
		return nil, fmt.Errorf("解码 %s 失败：%w", format, err)
	}

	bounds := src.Bounds()
	// 解码器返回的结果允许与头部声明不一致，因此再次校验。
	if err := limits.CheckPixelCount(bounds.Dx(), bounds.Dy()); err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	// image/draw 把所有来源类型（Paletted、YCbCr、Gray、NRGBA 等）统一成需要的 RGBA 布局，这是颜色转换必须正确的关键位置。
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)

	return &Image{
		Pix:    dst.Pix,
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Format: format,
	}, nil
}

// openRegular 仅当 path 是普通文件时才打开它。
// 目录、字符设备（COM1、CON）、FIFO 和符号链接都会被拒绝。
// 因为链接可能指向用户所选目录之外的位置，设备可能一直阻塞或者持续输出数据。
func openRegular(path string) (*os.File, error) {
	if path == "" {
		return nil, errors.New("未提供图片路径")
	}
	linkInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在：%s", path)
		}
		return nil, fmt.Errorf("无法访问文件：%w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("拒绝打开符号链接")
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, errors.New("只接受普通文件")
	}
	if linkInfo.Size() == 0 {
		return nil, errors.New("文件为空")
	}
	if linkInfo.Size() > limits.MaxFileBytes {
		return nil, fmt.Errorf("文件超过 %d MiB 上限", limits.MaxFileBytes>>20)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%w", err)
	}
	// 通过文件描述符再检查一次：路径在 Lstat 和 Open 之间可能被替换（TOCTOU 问题），而读取操作使用的是这个句柄。
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("读取文件状态失败：%w", err)
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("只接受普通文件")
	}
	if info.Size() > limits.MaxFileBytes {
		f.Close()
		return nil, fmt.Errorf("文件超过 %d MiB 上限", limits.MaxFileBytes>>20)
	}
	return f, nil
}

// detectFormat 按文件头部字节判断封装格式。判断过程不参考文件扩展名：
// 一个名为 photo.png 的文件如果内部是 JPEG 也可以接受，如果内部是其它内容则直接拒绝。
func detectFormat(fileBytes []byte) (Format, error) {
	switch {
	case len(fileBytes) >= 8 && bytes.Equal(fileBytes[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return FormatPNG, nil
	case len(fileBytes) >= 3 && fileBytes[0] == 0xff && fileBytes[1] == 0xd8 && fileBytes[2] == 0xff:
		return FormatJPEG, nil
	case len(fileBytes) >= 6 && (bytes.Equal(fileBytes[:6], []byte("GIF87a")) || bytes.Equal(fileBytes[:6], []byte("GIF89a"))):
		return FormatGIF, nil
	}
	return "", ErrUnsupportedFormat
}

func decodeConfig(f Format, fileBytes []byte) (image.Config, error) {
	r := bytes.NewReader(fileBytes)
	switch f {
	case FormatPNG:
		return png.DecodeConfig(r)
	case FormatJPEG:
		return jpeg.DecodeConfig(r)
	case FormatGIF:
		return gif.DecodeConfig(r)
	}
	return image.Config{}, ErrUnsupportedFormat
}

func decodeImage(f Format, fileBytes []byte) (image.Image, error) {
	r := bytes.NewReader(fileBytes)
	switch f {
	case FormatPNG:
		return png.Decode(r)
	case FormatJPEG:
		return jpeg.Decode(r)
	case FormatGIF:
		return gif.Decode(r) // 只取第一帧。
	}
	return nil, ErrUnsupportedFormat
}
