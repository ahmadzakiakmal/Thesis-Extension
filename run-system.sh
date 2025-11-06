# check if pwd has ./layer-1 and ./layer-2 directories
if [ -d "./layer-1" ] && [ -d "./layer-2" ]; then
 	echo "Running system with Layer 1 and Layer 2..."
 	cd layer-1
  make clean
 	make run
  cd ../layer-2
  make clean
 	make run
else
 	echo "Error: Please run this script from the root directory containing both layer-1 and layer-2 directories."
fi