package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/ocrclient"
	"github.com/denysvitali/odi/pkg/ocrclient/caroundtripper"
	"github.com/denysvitali/odi/pkg/ocrtext"
)

const (
	FlagOutputMode         = "output-mode"
	FlagFromJSON           = "from-json"
	FlagMergeDistance      = "merge-distance"
	FlagHorizontalDistance = "horizontal-distance"
)

var ocrCmd = &cobra.Command{
	Use:   "ocr [input-file]",
	Short: "Process an image through OCR, or convert OCR JSON output to text",
	Long: `Process an image file through the OCR API.

Output modes:
  - text: Plain text extracted from the image (default)
  - json: Full OCR result including bounding boxes and barcodes

If --from-json is set, the input-file is treated as a previously-captured
OCR JSON result and converted to plain text without contacting the OCR API.`,
	Args: cobra.ExactArgs(1),
	RunE: runOcr,
}

func init() {
	ocrCmd.Flags().StringP(FlagOutputMode, "o", "text", "Output mode: text or json")
	ocrCmd.Flags().BoolP(FlagDebug, "D", false, "Enable debug logging")
	ocrCmd.Flags().Bool(FlagFromJSON, false, "Treat input-file as an OCR JSON result and emit plain text")
	ocrCmd.Flags().Float64P(FlagMergeDistance, "m", ocrtext.DefaultMergeDistance, "Merge distance for text blocks")
	ocrCmd.Flags().Float64(FlagHorizontalDistance, ocrtext.DefaultHorizontalDistance, "Horizontal distance for column detection")

	AddOCRFlags(ocrCmd)
}

func runOcr(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	debug := GetBool(cmd, FlagDebug)
	log := logrus.StandardLogger()
	if debug {
		log.SetLevel(logrus.DebugLevel)
	}

	mergeDistance := GetFloat64(cmd, FlagMergeDistance)
	horizontalDistance := GetFloat64(cmd, FlagHorizontalDistance)

	if GetBool(cmd, FlagFromJSON) {
		v, err := parseOcrJson(inputPath)
		if err != nil {
			ui.PrintErrorf("Failed to parse JSON: %v", err)
			return err
		}
		fmt.Println(ocrtext.GetText(v, mergeDistance, horizontalDistance))
		return nil
	}

	if GetString(cmd, FlagOcrAPIAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OCR_API_ADDR)", FlagOcrAPIAddr)
	}

	c, err := ocrclient.New(GetString(cmd, FlagOcrAPIAddr))
	if err != nil {
		ui.PrintErrorf("Failed to create OCR client: %v", err)
		return err
	}

	if caPath := GetString(cmd, FlagOcrCaPath); caPath != "" {
		rt, err := caroundtripper.New(caPath)
		if err != nil {
			ui.PrintErrorf("Failed to create CA RoundTripper: %v", err)
			return err
		}
		c.SetHTTPTransport(rt)
	}

	f, err := os.Open(inputPath)
	if err != nil {
		ui.PrintErrorf("Failed to open file: %v", err)
		return err
	}
	defer f.Close()

	res, err := c.Process(cmd.Context(), f)
	if err != nil {
		ui.PrintErrorf("OCR processing failed: %v", err)
		return err
	}

	switch GetString(cmd, FlagOutputMode) {
	case "json":
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		if err := e.Encode(res); err != nil {
			ui.PrintErrorf("Failed to encode JSON: %v", err)
			return err
		}
	default:
		fmt.Print(ocrtext.GetText(res, mergeDistance, horizontalDistance))
	}
	return nil
}

func parseOcrJson(file string) (*ocrclient.OcrResult, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var v ocrclient.OcrResult
	err = dec.Decode(&v)
	return &v, err
}
