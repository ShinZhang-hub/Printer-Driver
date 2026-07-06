================================================================================
License Agreement
================================================================================

The license agreement for this software (hereinafter referred to as the
SOFTWARE) is described as follows.

1. Intellectual property rights in the SOFTWARE shall remain in
   FUJIFILM Business Innovation Corp. (hereinafter referred to as 
   FUJIFILM Business Innovation) as well as the original copyright holders.

2. The SOFTWARE can only be used with compatible FUJIFILM Business Innovation
   products (hereinafter referred to as the COMPATIBLE PRODUCTS) within the
   country of purchase of the COMPATIBLE PRODUCTS.

3. You are required to abide by the notes and restrictions (hereinafter referred
   to as the NOTES AND RESTRICTIONS) declared by FUJIFILM Business Innovation
   while using the SOFTWARE.

4. You are not permitted to alter, modify, reverse engineer, decompile or
   disassemble the whole or any part of the SOFTWARE for the purpose of
   analyzing the SOFTWARE.

5. You are not permitted to distribute the SOFTWARE on a communication network,
   or transfer, sell, rent or license the SOFTWARE to any third party by
   duplicating the SOFTWARE on any media such as floppy disk or magnetic tape.

6. FUJIFILM Business Innovation, FUJIFILM Business Innovation Channel Partners,
   Authorized Dealers and the original copyright holders of the SOFTWARE shall
   not be liable for any loss or damage arising from matching of hardware or 
   program that are not specified in the NOTES AND RESTRICTIONS of the SOFTWARE,
   or any modification to the SOFTWARE.

7. FUJIFILM Business Innovation, FUJIFILM Business Innovation Channel Partners,
   Authorized Dealers and the original copyright holders of the SOFTWARE shall
   not be responsible for any warranty or liability with respect to the SOFTWARE.

================================================================================
ART EX Print Driver Ver.7.1.8
================================================================================

This document provides information about the driver on the
following items:

1. Target Hardware Products
2. Requirements
3. Installation
4. Version Improvements
5. NOTES AND RESTRICTIONS
  5.1 Driver-Specific NOTES AND RESTRICTIONS
  5.2 Driver-Specific NOTES AND RESTRICTIONS for Color
  5.3 Application-Specific NOTES AND RESTRICTIONS
  5.4 Application-Specific NOTES AND RESTRICTIONS for Color
6. Inquiries

---------------------------------------------------
1. Target Hardware Products
---------------------------------------------------
FUJIFILM Apeos 3060 / 2560 / 1860
         Apeos 4570 / 3570
         Apeos 5330
         Apeos 6340
         Apeos 7580 / 6580 / 5580
         Apeos C2360 / C2060
         Apeos C3061 / C2561 / C2061
         Apeos C3067
         Apeos C4030 / C3530
         Apeos C5240
         Apeos C7070 / C6570 / C5570 / C4570 / C3570 / C3070 / C2570
         Apeos C7071 / C6571 / C5571 / C4571 / C3571 / C2571
         Apeos C8180 / C7580 / C6580
         ApeosPrint 4560 S / 3960 S / 3360 S
         ApeosPrint 4830
         ApeosPrint 4830 JM
         ApeosPrint 6340
         ApeosPrint C3560 S / C3060 S
         ApeosPrint C4030 / C3530
         ApeosPrint C5240
         ApeosPrint C5570 / C4570
         ApeosPro C810 / C750 / C650

---------------------------------------------------
2. Requirements
---------------------------------------------------
Please note that this driver operates on a computer running on the following
Japanese/English operating system.

  Microsoft(R) Windows(R) 10 x64 Editions
  Microsoft(R) Windows Server(R) 2016
  Microsoft(R) Windows Server(R) 2019
  Microsoft(R) Windows(R) 11
  Microsoft(R) Windows Server(R) 2022
  Microsoft(R) Windows Server(R) 2025

  Click the following URL to see the latest update on OS support.
  https://fujifilm.com/fb/download/defacto

---------------------------------------------------
3. Installation
---------------------------------------------------
Double-click the installer (Launcher.exe) to launch the installer.
Follow the on-screen instructions of the installer to complete the installation.
If install the printer driver using Add Printer, refer to the following URL.
https://fujifilm.com/fb/download/info/manual_inst.html

---------------------------------------------------
4. Version Improvements
---------------------------------------------------
[Improvements in Ver 7.1.4]
* The drivers for the products listed in "1.Target Hardware Products" have been
  combined into a single package.

  The improvements made so far for each model are described in releasenote_English.txt.

[Improvements in Ver 7.1.5]
* In Apeos C3067,and Apeos C3061 / C2561 / C2061,
  [C5 Envelope] has been added to the list of paper sizes that can be specified.
* Fixed an issue that the upgrade to Ver 7.1.4 failed.

[Improvements in Ver 7.1.8]
* Removed unnecessary explanatory text in the help section on [EMF Spooling] 
  on the [Advanced] tab.
* Added the [Face Up / Down Output] function.
  The target models are as follows.
    Apeos 7580 / 6580 / 5580
    Apeos C8180 / C7580 / C6580
    ApeosPro C810 / C750 / C650
* Added the [Restriction of Job Type] function so that the [Job Type]
  can be fixed to [Secure Print].
  The target models are as follows. 
    Apeos 3060 / 2560 / 1860
    Apeos 4570 / 3570
    Apeos 5330
    Apeos 6340
    Apeos 7580 / 6580 / 5580
    Apeos C2360 / C2060
    Apeos C4030 / C3530
    Apeos C5240
    Apeos C7070 / C6570 / C5570 / C4570 / C3570 / C3070 / C2570
    Apeos C8180 / C7580 / C6580
    ApeosPrint 4560 S / 3960 S / 3360 S
    ApeosPrint 4830
    ApeosPrint 4830 JM
    ApeosPrint 6340
    ApeosPrint C4030 / C3530
    ApeosPrint C5240
    ApeosPrint C5570 / C4570
    ApeosPro C810 / C750 / C650
   The following models are supported by the first version (Ver 7.1.4).
    Apeos C3061 / C2561 / C2061
    Apeos C3067
    Apeos C7071 / C6571 / C5571 / C4571 / C3571 / C2571


---------------------------------------------------
5. NOTES AND RESTRICTIONS
---------------------------------------------------
5.1 Driver-Specific NOTES AND RESTRICTIONS
-----------------------------------------------
* Printer driver resolution
  The default value of the resolution of this driver is set to 600dpi
  (automatic on display). Depending on the specification and limitations of the
  application, the following variations in print result may occur when you
  output from a driver set to a resolution that is different from this driver.
  - The print layout of the document is changed.
  - The print result of lines and patterns is different.
  - Unnecessary lines are drawn in the print result.
  - Necessary lines are not drawn in the print result.
  When this happens, the print result may be improved by changing the
  [Resolution] setting on the [Advanced] tab.

* Notes and limitations about the server-client environment
- Server-Client Environment Issue (1)
  If the printer is being used as a shared printer and the server's operating
  system is being upgraded, a message indicating that the driver has to be
  updated may appear on the client, causing printing to fail.
  In this case, the print driver has to be reinstall on the client to be able
  to print again.

- Server-Client Environment Issue (2)
  In the Server-Client environment, after a print driver is added or upgraded
  on the Server side, printing may not be performed with a displayed message
  requiring print driver upgrade on the Client side.
  This problem can be avoided by the settings below.

  < Change of Group Policy Settings on Client PC >
  1. Log on as an Administrator on Client PC.
  2. Open command prompt and execute "gpedit.msc". to open [Local Group Policy
     Editor].
  3. Open the tree on the left side in the following order.
     [User Configuration]
     [Administrative Templates]
     [Control Panel]
     [Printers]
  4. Double-click [Point and Print Restrictions] in the right pane.
  5. Click the [Disabled] radio button.
  6. Click [OK] to close the window.
  7. Close [Local Group Policy Editor].

* About the use of the shared printer
  If any of the following resulted under the shared printer environment,
  it may be printed properly by changing the value of
  [Render print jobs on client computers].
  - Annotation/ Watermark is not printing properly, even with these specified.
  - The authentication setting is not reflected properly, or the authentication
    pop-up does not appear.

* Skip Blank Pages
  Even with [Skip Blank Pages] selected, blank pages may still be printed in
  the following situations.
  - The page contains only line feeds.
  - The page contains only spaces.
  - The page contains only line feeds and spaces.
  - A white background drawing instruction is sent from the application.

5.2 Driver-Specific NOTES AND RESTRICTIONS for Color
------------------------------------------------------
* About [Prioritize Application Specified Color when Output Color is Black & White]
  For print output that is consistent with color settings specified by application UI,
  set both [Print Using Application Specified Color]
  and [Prioritize Application Specified Color when Output Color is Black & White] to [On].

  The following describes the relationship between printer driver settings
  ([Output Color], [Print Using Application Specified Color], and
  [Prioritize Application Specified Color when Output Color is Black & White])
  and print output.

  * [Print Using Application Specified Color]: [Off]
    Output Color specified by the application will be ignored, and driver UI's
    [Output Color] setting will be used for printing.

  * [Print Using Application Specified Color]: [On]
    - [Prioritize Application Specified Color when Output Color is Black & White]: [On]
      Output Color specified by the application will be used for printing.
      If no Output Color has been specified by the application, driver UI's
      [Output Color] setting will be used for printing.

    - [Prioritize Application Specified Color when Output Color is Black & White]: [Off]
      If driver UI's [Output Color] setting is [Color], Output Color specified
      by the application will be used for printing.
      If driver UI's [Output Color] setting is [Black and White], Output Color
      specified by the application will be ignored, and print output will be in
      black and white.
      If no Output Color has been specified by the application, driver UI's
      [Output Color] setting will be used for printing.

* Black/Color Auto Recognition
  When [Color] is specified at [Output Color], though the output may appear to
  be in black, it has been processed in color.
  If you are sure that you want to print in black, specify [Black] at [Output
  Color]. Documents will be printed in color in the following situations.
  - When black and white objects are overlapping the color objects
  - When there are color objects outside the print area
  - When the application uses the system's ICM function to perform color change
  - When the application has its own color change function

5.3 Application-Specific NOTES AND RESTRICTIONS
---------------------------------------------------
* All Applications
  - Depending on the application used by the customer, blank pages for page
    adjustment will be inserted automatically according to the conditions like
    the number of copies specified when outputting 2-sided prints.
    In this case, the blank inserts will be included by the application.
    The performance is improved by changing the setting below.
    - Set [Skip Blank Pages] on [Advanced] tab to [On].

* Internet Explorer mode in Microsoft Edge
  Since File creation at printing is restricted on Internet Explorer mode in 
  Microsoft Edge in Protection mode Form file can not be created even if 
  [Create Background Forms] is specified.

  When outputting from Internet Explorer mode in Microsoft Edge in Protection
  mode changes made in the pop-up [Enter User Details] dialog are not taken 
  over to the output next time.

5.4 Application-Specific NOTES AND RESTRICTIONS for Color
-------------------------------------------------------------
* Output Color Settings from Application
  To print in color from Application of Windows Store, open [Devices and Printers]
  from Windows Desktop, right-click on your printer to select [Printing Preferences]
  and then confirm that [Output Color] on the [Basic] tab of the [Printing Preferences]
  dialog is set to [Color].
  If the setting of [Output Color] on the [Basic] tab of the [Printing Preferences]
  dialog remains [Black and White], the output will be in black and white even
  if you specify Color Mode to [Color] on the printing setting screen shown
  after you select printer from the device charm.

---------------------------------------------------
6. Inquiries
---------------------------------------------------
FUJIFILM Business Innovation Corp.
Customer Contact Center or Printer Support Desk

Please check the telephone number to call on the
"Maintenance/Operation Inquiries" card pasted on the machine.

The latest software is available on our web site.
   https://fujifilm.com/fb/download
The charges shall be borne by the customers.

--------------------------------------------------------------------------------
Microsoft, Windows, Windows Server, Internet Explorer and Microsoft Edge
are either registered trademarks or trademarks of Microsoft Corporation
in the United States and/or other countries.

Other company names and product names are trademarks or registered
trademarks of the respective companies.

(C) FUJIFILM Business Innovation Corp. 2021-2025
