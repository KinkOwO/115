// sub_140B899B0  va=0x140B899B0  size=248

__int64 __fastcall sub_140B899B0(__int64 a1, char a2, unsigned __int16 a3)
{
  int v3; // esi
  __int64 v5; // rax
  int v6; // r9d
  __int64 v7; // rdi
  __int64 v8; // rcx
  __int64 v9; // rax
  __int64 v10; // rax

  v3 = a3;
  v5 = sub_14148BF80();
  v7 = v5;
  if ( a2 != 0 )
  {
    if ( v5 != 0 )
      sub_141491360(v5);
    goto LABEL_20;
  }
  switch ( v3 )
  {
    case 1:
      if ( *(_QWORD *)&qword_14E683C78 == 0 )
        goto LABEL_18;
      v8 = 100087775;
      goto LABEL_17;
    case 3:
      if ( *(_QWORD *)&qword_14E683C78 == 0 )
        goto LABEL_18;
      v8 = 6408;
      goto LABEL_17;
    case 119:
      if ( *(_QWORD *)&qword_14E683C78 == 0 )
        goto LABEL_18;
      v8 = 400002881;
      goto LABEL_17;
    default:
      break;
  }
  if ( v3 != 217 )
  {
    LOBYTE(v6) = 1;
    sub_146ADFC80(v3, 0, 1, v6, 1, 1);
    goto LABEL_18;
  }
  if ( *(_QWORD *)&qword_14E683C78 != 0 )
  {
    v8 = 101039466;
LABEL_17:
    v9 = sub_14723C170(v8);
    sub_14668C520(*(_QWORD *)&qword_14E683C78, 2875, v9, 0);
  }
LABEL_18:
  if ( v7 != 0 )
    sub_1414921F0(v7, 4);
LABEL_20:
  v10 = sub_140B915D0();
  return sub_140B96DB0(v10, 0);
}
