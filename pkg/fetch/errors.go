// Copyright © 2018 The Things Network Foundation, The Things Industries B.V.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fetch

import "go.thethings.network/lorawan-stack/pkg/errors"

var (
	errFileNotFound      = errors.DefineNotFound("file_not_found", "file `{filename}` not found")
	errCouldNotFetchFile = errors.Define("fetch_file", "could not fetch file `{filename}`")
	errCouldNotReadFile  = errors.DefineCorruption("read_file", "could not read file `{filename}`")
)
