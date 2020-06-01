/*
Used by the block handler to periodically consolidate individual block files into 1000 block file units.
*/
package block_handler

import (
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"os"
)

// Periodical check whether we can consolidate individual block files.
func (s *state) consolidateBlockFiles() {
	files, err := getIndividualBlockFilesSorted()
	if err != nil {
		logging.ErrorLog.Print(err.Error())
		return
	}

	var consolidationThreshold int64
	// this is already the new method introduced in v565
	if s.retentionEdgeHeight >= 0 {
		consolidationThreshold = s.retentionEdgeHeight
	} else {
		consolidationThreshold = s.frozenEdgeHeight - int64(s.ctxt.CycleAuthority.GetCurrentCycleLength()*5)
	}
	currentFileIndex := consolidationThreshold / blocksPerFile
	consolidationMap := make(map[int64][]string)
	for _, file := range files {
		blockHeight := blockHeightForIndividualFile(file.Name())
		if blockHeight > 0 {
			fileIndex := blockHeight / blocksPerFile
			if fileIndex < currentFileIndex {
				consolidationMap[fileIndex] = append(consolidationMap[fileIndex], file.Name())
			}
		}
	}
	for fileIndex, fileList := range consolidationMap {
		var err error
		if s.blockConsolidatorOption != configuration.BlockFileConsolidatorOptionDelete {
			err = s.consolidateBlockFilesForIndex(fileIndex)
		}
		if err != nil {
			logging.ErrorLog.Print(err.Error())
		} else {
			s.deleteBlockFiles(fileList)
		}
	}
}

// Consolidate individual blocks to a consolidated block file. This is mighty different from the Java version, but
// should lead to the same results.
func (s *state) consolidateBlockFilesForIndex(fileIndex int64) error {
	startBlockHeight := fileIndex * blocksPerFile
	blocks, _ := s.GetBlocks(startBlockHeight, startBlockHeight+blocksPerFile-1)
	if blocks == nil || len(blocks) == 0 {
		return nil
	}

	var serialized []byte
	successful := true
	serialized = append(serialized, message_fields.SerializeInt16(int16(len(blocks)))...)
	for index, block := range blocks {
		serialized = append(serialized, block.ToBytes()...)
		if index == 0 || blocks[index-1].Height != block.Height-1 {
			balanceList := s.GetBalanceList(block.Height)
			if balanceList != nil {
				serialized = append(serialized, balanceList.ToBytes()...)
			} else {
				successful = false
				break
			}
		}
	}

	if successful {
		consolidatedPath := consolidatedPathForBlockHeight(blocks[0].Height)
		consolidatedFileName := consolidatedFileForBlockHeight(blocks[0].Height)
		tempFileName := consolidatedFileName + "_tmp"

		err := os.MkdirAll(consolidatedPath, os.ModePerm)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not create directory %s for consolidated block file.", consolidatedPath))
		}
		_ = os.Remove(consolidatedFileName)
		_ = os.Remove(tempFileName)
		out, err := os.Create(tempFileName)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not create temporary consolidated block file %s.", tempFileName))
		}
		_, err = out.Write(serialized)
		if err != nil {
			out.Close()
			return errors.New(fmt.Sprintf("Could not write temporary consolidated block file %s.", tempFileName))
		}
		out.Close()
		err = os.Rename(tempFileName, consolidatedFileName)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not move temporary consolidated block file %s to permanent location %s.", tempFileName, consolidatedFileName))
		}
		logging.InfoLog.Printf("Consolidated %d blocks to file %s.", len(blocks), consolidatedFileName)
	}
	return nil
}

func (s *state) deleteBlockFiles(fileList []string) {
	for _, file := range fileList {
		_ = os.Remove(configuration.DataDirectory + "/" + configuration.IndividualBlockDirectory + "/" + file)
	}
	logging.InfoLog.Printf("Deleted %d block files in BlockFileConsolidator.", len(fileList))
}
