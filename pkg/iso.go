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

func MkEFIBootloader(out, linuxBootFilepath, shimFilepath, mmxFilepath string) error {
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
		padding          int64  = 5 * 1024 * 1024
		align1MiBMask    uint64 = (1<<44 - 1) << 20
		partSize         int64  = int64(uint64(espSize) & align1MiBMask)
		diskSize         int64  = (partSize * 2) + padding
		partitionSectors int64  = partSize / int64(diskfs.SectorSize512)
		partition1Start  int64  = 2048
		partition1End    int64  = partitionSectors - partition1Start + 1
		partition2Start  int64  = partition1End + 1
		partition2End    int64  = ((diskSize - padding) / int64(diskfs.SectorSize512)) - partition2Start + 1
	)

	disk, err := diskfs.Create(out, diskSize, diskfs.Raw, diskfs.SectorSize512)
	if err != nil {
		return fmt.Errorf("failed to create disk file: %w", err)
	}
	disk.LogicalBlocksize = 2048
	table := &gpt.Table{
		Partitions: make([]*gpt.Partition, 2),
	}
	// TODO Attributes:
	table.Partitions[0] = &gpt.Partition{
		Start: uint64(partition1Start),
		End:   uint64(partition1End),
		Type:  gpt.MicrosoftBasicData,
		Name:  "ISO9660",
	}
	table.Partitions[1] = &gpt.Partition{
		Start: uint64(partition2Start),
		End:   uint64(partition2End),
		Type:  gpt.EFISystemPartition,
		Name:  "EFI System",
	}
	err = disk.Partition(table)
	if err != nil {
		return fmt.Errorf("failed to create partitiont table: %w", err)
	}
	iso := diskpkg.FilesystemSpec{Partition: 0, FSType: filesystem.TypeISO9660, VolumeLabel: volumeLabel}
	part1, err := disk.CreateFilesystem(iso)
	if err != nil {
		return fmt.Errorf("failed to create iso9660 filesystem")
	}
	if err := writeFilesToFs(part1, shimFilepath, "/EFI/boot/bootx64.efi"); err != nil {
		return fmt.Errorf("failed to write shim to iso: %w", err)
	}
	if err := writeFilesToFs(part1, mmxFilepath, "/EFI/boot/mmx64.efi"); err != nil {
		return fmt.Errorf("failed to write mmx to iso: %w", err)
	}
	if err := writeFilesToFs(part1, linuxBootFilepath, "/EFI/boot/linuxboot.efi"); err != nil {
		return fmt.Errorf("failed to write linuxboot to iso: %w", err)
	}
	isoImage, ok := part1.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("not an iso9660 filesystem")
	}
	options := iso9660.FinalizeOptions{
		VolumeIdentifier: volumeIdentifier,
		ElTorito: &iso9660.ElTorito{
			BootCatalog: "/BOOT.CAT",
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.EFI,
					Emulation: iso9660.NoEmulation,
					BootFile:  "/EFI/boot/bootx64.efi",
				},
			},
		},
	}
	if err = isoImage.Finalize(options); err != nil {
		return err
	}
	fat := diskpkg.FilesystemSpec{Partition: 1, FSType: filesystem.TypeFat32}
	part2, err := disk.CreateFilesystem(fat)
	if err != nil {
		return fmt.Errorf("failed to create vfat filesystem")
	}
	if err := writeFilesToFs(part2, shimFilepath, "/EFI/boot/bootx64.efi"); err != nil {
		return fmt.Errorf("failed to write shim: %w", err)
	}
	if err := writeFilesToFs(part2, mmxFilepath, "/EFI/boot/mmx64.efi"); err != nil {
		return fmt.Errorf("failed to write mmx: %w", err)
	}
	if err := writeFilesToFs(part2, linuxBootFilepath, "/EFI/boot/linuxboot.efi"); err != nil {
		return fmt.Errorf("failed to write linuxboot: %w", err)
	}
	return nil
}
