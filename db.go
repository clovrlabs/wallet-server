package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/breez/server/breez"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	TypeUnknown = iota
	TypeBoltzReverseSwapLockup
)

const (
	StatusUnknown = iota
	StatusUnconfirmed
	StatusNotified
)

type BoltzReverseSwapInfo struct {
	ID                 string `json:"id"`
	TimeoutBlockHeight uint32 `json:"timeout_block_height"`
}

var (
	pgxPool *pgxpool.Pool
)

func pgConnect() error {
	var err error
	pgxPool, err = pgxpool.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("pgxpool.Connect(%v): %w", os.Getenv("DATABASE_URL"), err)
	}
	return nil
}

func insertTxNotification(in *breez.PushTxNotificationRequest) (*uuid.UUID, error) {
	u, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("uuid.NewRandom(): %w", err)
	}
	var additionalInfo []byte
	var txType int32
	switch x := in.Info.(type) {
	case *breez.PushTxNotificationRequest_BoltzReverseSwapLockupTxInfo:
		additionalInfo, _ = json.Marshal(BoltzReverseSwapInfo{
			ID:                 x.BoltzReverseSwapLockupTxInfo.BoltzId,
			TimeoutBlockHeight: x.BoltzReverseSwapLockupTxInfo.TimeoutBlockHeight,
		})
		txType = TypeBoltzReverseSwapLockup
	default:
		txType = TypeUnknown
	}
	commandTag, err := pgxPool.Exec(context.Background(),
		`INSERT INTO tx_notifications
		  (id, tx_type, status, additional_info, title, body, device_id, tx_hash, script, block_height_hint)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		  ON CONFLICT (device_id, script) DO NOTHING`,
		pgtype.UUID{Bytes: u, Status: pgtype.Present},
		txType,
		StatusUnconfirmed,
		additionalInfo,
		in.Title,
		in.Body,
		in.DeviceId,
		in.TxHash,
		in.Script,
		in.BlockHeightHint,
	)
	if err != nil {
		log.Printf("pgxPool.Exec(): %v", err)
		return nil, fmt.Errorf("pgxPool.Exec(): %w", err)
	}
	log.Printf("pgxPool.Exec('INSERT INTO tx_notification()'; RowsAffected(): %v'", commandTag.RowsAffected())
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return &u, nil
}

func txNotified(u uuid.UUID, txHash chainhash.Hash, tx []byte, blockHeigh uint32, blockHash []byte, txIndex uint32) error {
	commandTag, err := pgxPool.Exec(context.Background(),
		`UPDATE tx_notifications
		 SET status = $2, tx_hash=$3, tx=$4, block_height=$5, block_hash=$6, tx_index=$7
		 WHERE id=$1`,
		u, StatusNotified, txHash.CloneBytes(), tx, blockHeigh, blockHash, txIndex,
	)
	if err != nil {
		log.Printf("pgxPool.Exec(): %v", err)
		return fmt.Errorf("pgxPool.Exec(): %w", err)
	}
	log.Printf("pgxPool.Exec('UPDATE tx_notifications'; RowsAffected(): %v'", commandTag.RowsAffected())
	return nil
}

func boltzReverseSwapToNotify(currentHeight uint32) (pgx.Rows, error) {
	return pgxPool.Query(context.Background(),
		`SELECT id, additional_info, title, body, device_id, tx_hash, script, block_height_hint
		 FROM tx_notifications tn
		 WHERE tn.tx_type=$1 AND tn.status=$2 AND cast(tn.additional_info->>'timeout_block_height' as int)>$3`,
		TypeBoltzReverseSwapLockup, StatusUnconfirmed, currentHeight,
	)
}