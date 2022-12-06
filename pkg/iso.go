package isogen

import (
	"fmt"
	"os"
	"path/filepath"

	diskfs "github.com/diskfs/go-diskfs"
	diskpkg "github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

const (
	volumeLabel      = "IMMU"
	volumeIdentifier = "immu"
)

func writeFilesToFs(fs filesystem.FileSystem, file, diskPath string) error {

	if path := filepath.Dir(diskPath); path != "/" {
		if err := fs.Mkdir(path); err != nil {
			return err
		}
	}
	rw, err := fs.OpenFile(diskPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("failed to make %s on the disk image", diskPath)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s", filepath.Base(file))
	}
	_, err = rw.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write %s", filepath.Base(file))
	}
	return nil
}

func Mkvfat(out, linuxBootFilepath, shimFilepath, mmxFilepath string) error {
	var espSize int64
	for _, file := range []string{shimFilepath, linuxBootFilepath} {
		if file != "" {
			fi, err := os.Stat(file)
			if err != nil {
				return nil
			}
			espSize += fi.Size()
		}
	}

	var (
		align1MiBMask    uint64 = (1<<44 - 1) << 20
		partSize         int64  = int64(uint64(espSize) & align1MiBMask)
		diskSize         int64  = partSize + 5*1024*1024
		partitionStart   int64  = 2048
		partitionSectors int64  = partSize / int64(diskfs.SectorSize512)
		partitionEnd     int64  = partitionSectors - partitionStart + 1
	)

	disk, err := diskfs.Create(out, diskSize, diskfs.Raw, diskfs.SectorSize512)
	if err != nil {
		return fmt.Errorf("failed to create disk file: %w", err)
	}
	table := &gpt.Table{
		Partitions: make([]*gpt.Partition, 1),
	}
	table.Partitions[0] = &gpt.Partition{
		Start: uint64(partitionStart),
		End:   uint64(partitionEnd),
		Type:  gpt.EFISystemPartition,
		Name:  "EFI System",
	}
	err = disk.Partition(table)
	if err != nil {
		return fmt.Errorf("failed to create partitiont table: %w", err)
	}
	spec := diskpkg.FilesystemSpec{Partition: 0, FSType: filesystem.TypeFat32}
	fs, err := disk.CreateFilesystem(spec)
	if err != nil {
		return fmt.Errorf("failed to create filesystem")
	}
	if err := writeFilesToFs(fs, shimFilepath, "/EFI/boot/bootx64.efi"); err != nil {
		return fmt.Errorf("failed to write shim: %w", err)
	}
	if err := writeFilesToFs(fs, mmxFilepath, "/EFI/boot/mmx64.efi"); err != nil {
		return fmt.Errorf("failed to write mmx: %w", err)
	}
	if err := writeFilesToFs(fs, linuxBootFilepath, "/EFI/boot/linuxboot.efi"); err != nil {
		return fmt.Errorf("failed to write linuxboot: %w", err)
	}
	return nil
}

func Mkiso(out, vfat string) error {
	fi, err := os.Stat(vfat)
	if err != nil {
		return err
	}
	size := fi.Size()
	size = size + 5*1024*1024 // disk padding
	iso, err := diskfs.Create(out, size, diskfs.Raw, diskfs.SectorSize512)
	if err != nil {
		return err
	}
	iso.LogicalBlocksize = 2048
	fs, err := iso.CreateFilesystem(diskpkg.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: volumeLabel,
	})
	if err != nil {
		return err
	}
	// This avoids an issue where path.Base in go-diskfs gives us a sigsegv
	vfatName := filepath.Join("vfat", filepath.Base(vfat))
	if err := writeFilesToFs(fs, vfat, vfatName); err != nil {
		return fmt.Errorf("failed to write file %s to ISO: %w", vfat, err)
	}
	diskImage, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("not an iso9660 filesystem")
	}
	options := iso9660.FinalizeOptions{
		VolumeIdentifier: volumeIdentifier,
		ElTorito: &iso9660.ElTorito{
			BootCatalog: "/boot.catalog",
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.EFI,
					Emulation: iso9660.NoEmulation,
					BootFile:  vfatName,
				},
			},
		},
	}
	if err = diskImage.Finalize(options); err != nil {
		return err
	}
	return nil
}
